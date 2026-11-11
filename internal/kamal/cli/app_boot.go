package cli

// Ports lib/kamal/cli/app/boot.rb, assets.rb, error_pages.rb, ssl_certificates.rb,
// and lib/kamal/cli/healthcheck/ at Kamal 2.12.0.

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/commands"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/sshexec"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/tags"
)

// healthcheckError reports a container that never became healthy.
type healthcheckError struct{ message string }

func (e *healthcheckError) Error() string { return e.message }

// RubyClass names the Kamal exception.
func (e *healthcheckError) RubyClass() string { return "Kamal::Cli::Healthcheck::Error" }

// barrier lets the first healthy primary container release the other roles, or
// stop them when it fails. The first open or close wins.
type barrier struct {
	once   sync.Once
	done   chan struct{}
	opened bool
}

func newBarrier() *barrier { return &barrier{done: make(chan struct{})} }

func (b *barrier) set(value bool) bool {
	won := false
	b.once.Do(func() {
		b.opened, won = value, true
		close(b.done)
	})
	return won
}

func (b *barrier) open() bool  { return b.set(true) }
func (b *barrier) close() bool { return b.set(false) }

func (b *barrier) wait() error {
	<-b.done
	if !b.opened {
		return &healthcheckError{message: "Halted at barrier"}
	}
	return nil
}

// boot boots one role on one host, as Kamal::Cli::App::Boot does.
type boot struct {
	c       *CLI
	s       *sshexec.Session
	role    *configuration.Role
	version string
	barrier *barrier
	app     *commands.App
}

func (c *CLI) newBoot(s *sshexec.Session, role *configuration.Role, version string, b *barrier) *boot {
	return &boot{c: c, s: s, role: role, version: version, barrier: b, app: c.kamal.App(role, s.Host)}
}

func (b *boot) run() error {
	oldVersion, err := b.oldVersionRenamedIfClashing()
	if err != nil {
		return err
	}
	if b.queuer() {
		if err := b.waitAtBarrier(); err != nil {
			return err
		}
	}
	if err := b.startNewVersion(); err != nil {
		return b.failedStart(err)
	}
	if b.gatekeeper() {
		b.releaseBarrier()
	}
	if oldVersion != "" {
		return b.stopOldVersion(oldVersion)
	}
	return nil
}

func (b *boot) failedStart(err error) error {
	if b.gatekeeper() {
		b.closeBarrier()
	}
	if stopErr := b.s.Execute(lenientCmd(b.app.Stop(b.version))); stopErr != nil {
		return stopErr
	}
	return err
}

func (b *boot) oldVersionRenamedIfClashing() (string, error) {
	existing, err := b.s.CaptureWithInfo(lenientCmd(b.app.ContainerIDForVersion(b.version, false)))
	if err != nil {
		return "", err
	}
	if existing != "" {
		if err := b.renameClashingContainer(); err != nil {
			return "", err
		}
	}
	current, err := b.s.CaptureWithInfo(lenientCmd(b.app.CurrentRunningVersion()))
	return strings.TrimSpace(current), err
}

func (b *boot) renameClashingContainer() error {
	renamed := b.version + "_replaced_" + b.c.rt.RandomHex(8)
	b.s.Log(sshexec.Info, "Renaming container "+b.version+" to "+renamed+" as already deployed on "+b.s.Host)
	if err := b.audit("Renaming container " + b.version + " to " + renamed); err != nil {
		return err
	}
	return b.s.Execute(cmd(b.app.RenameContainer(b.version, renamed)))
}

func (b *boot) startNewVersion() error {
	err := b.startNewVersionSteps()
	if err != nil {
		b.s.Log(sshexec.Error, "Failed to boot "+b.role.Name()+" on "+b.s.Host)
	}
	return err
}

func (b *boot) startNewVersionSteps() error {
	if err := b.audit("Booted app version " + b.version); err != nil {
		return err
	}
	if err := b.s.Execute(cmd(b.app.EnsureEnvDirectory())); err != nil {
		return err
	}
	secrets, err := b.role.SecretsFile(b.s.Host)
	if err != nil {
		return err
	}
	if err := b.s.Upload(sshexec.Upload{Content: []byte(secrets), InMemory: true, Mode: "0600"}, b.role.SecretsPath()); err != nil {
		return err
	}
	if err := b.s.Execute(cmd(b.app.Run(b.hostname()))); err != nil {
		return err
	}
	return b.waitUntilServing()
}

// hostname is the host name truncated to 51 characters, without a trailing dot,
// with a random suffix.
func (b *boot) hostname() string {
	host := b.s.Host
	if len(host) > 51 {
		host = host[:51]
	}
	return strings.TrimSuffix(host, ".") + "-" + b.c.rt.RandomHex(6)
}

func (b *boot) waitUntilServing() error {
	if !b.role.RunningProxy() {
		return b.c.waitForHealthy(b.s, func() (string, error) { return b.s.CaptureWithInfo(cmd(b.app.Status(b.version))) })
	}
	endpoint, err := b.s.CaptureWithInfo(cmd(b.app.ContainerIDForVersion(b.version, false)))
	if err != nil {
		return err
	}
	if endpoint == "" {
		return bootError("Failed to get endpoint for " + b.role.Name() + " on " + b.s.Host + ", did the container boot?")
	}
	return b.s.Execute(cmd(b.app.Deploy(endpoint)))
}

func (b *boot) stopOldVersion(version string) error {
	if err := b.s.Execute(lenientCmd(b.app.Stop(version))); err != nil {
		return err
	}
	if b.role.Assets() {
		if err := b.s.Execute(cmd(b.app.CleanUpAssets())); err != nil {
			return err
		}
	}
	if b.c.config().ErrorPagesPath().Truthy() {
		return b.s.Execute(cmd(b.app.CleanUpErrorPages()))
	}
	return nil
}

func (b *boot) primaryRoleName() string { return b.c.kamal.PrimaryRole().Name() }

func (b *boot) releaseBarrier() {
	if b.barrier.open() {
		b.s.Log(sshexec.Info, "First "+b.primaryRoleName()+" container is healthy on "+b.s.Host+", booting any other roles")
	}
}

func (b *boot) waitAtBarrier() error {
	b.s.Log(sshexec.Info, "Waiting for the first healthy "+b.primaryRoleName()+" container before booting "+b.role.Name()+" on "+b.s.Host+"...")
	if err := b.barrier.wait(); err != nil {
		b.s.Log(sshexec.Info, "First "+b.primaryRoleName()+" container is unhealthy, not booting "+b.role.Name()+" on "+b.s.Host)
		return err
	}
	b.s.Log(sshexec.Info, "First "+b.primaryRoleName()+" container is healthy, booting "+b.role.Name()+" on "+b.s.Host+"...")
	return nil
}

// closeBarrier stops the other roles and logs why the primary container failed.
func (b *boot) closeBarrier() {
	if !b.barrier.close() {
		return
	}
	b.s.Log(sshexec.Info, "First "+b.primaryRoleName()+" container is unhealthy on "+b.s.Host+", not booting any other roles")
	logs, err := b.s.CaptureWithInfo(cmd(b.app.Logs(commands.LogOptions{ContainerCommand: b.app.ContainerIDForVersion(b.version, false), Timestamps: true})))
	if err == nil {
		b.s.Log(sshexec.Error, logs)
		var health string
		health, err = b.s.CaptureWithInfo(cmd(b.app.ContainerHealthLog(b.version)))
		b.s.Log(sshexec.Error, health)
	}
	var failed *sshexec.FailedError
	if errors.As(err, &failed) {
		b.s.Log(sshexec.Error, "Could not fetch logs for "+b.version)
	}
}

func (b *boot) barrierRole() bool { return b.role == b.c.kamal.PrimaryRole() }
func (b *boot) gatekeeper() bool  { return b.barrier != nil && b.barrierRole() }
func (b *boot) queuer() bool      { return b.barrier != nil && !b.barrierRole() }

func (b *boot) audit(message string) error {
	return b.s.Execute(debugCmd(b.c.kamal.Auditor(tags.Tag{Name: "role", Value: b.role.Name()}).Record(message)))
}

// waitForHealthy polls the container status until it is running or healthy, or
// until deploy_timeout passes, as Kamal's healthcheck poller does.
func (c *CLI) waitForHealthy(s *sshexec.Session, status func() (string, error)) error {
	timeout := c.config().DeployTimeout().Int()
	deadline := c.rt.Now().Add(time.Duration(timeout) * time.Second)
	for attempt := 1; ; attempt++ {
		err := c.checkHealthy(s, status, timeout)
		var unhealthy *healthcheckError
		if !errors.As(err, &unhealthy) {
			if err == nil {
				s.Log(sshexec.Info, "Container is healthy!")
			}
			return err
		}
		left := deadline.Sub(c.rt.Now())
		if left <= 0 {
			return err
		}
		c.rt.Sleep(min(time.Duration(attempt)*time.Second, left))
	}
}

func (c *CLI) checkHealthy(s *sshexec.Session, status func() (string, error), timeout int64) error {
	current, err := status()
	if err != nil {
		return err
	}
	if delay := c.config().ReadinessDelay(); current == "running" && delay > 0 {
		s.Log(sshexec.Info, fmt.Sprintf("Container is running, waiting for readiness with a delay of %d seconds", delay))
		c.rt.Sleep(time.Duration(delay) * time.Second)
		if current, err = status(); err != nil {
			return err
		}
	}
	if current != "running" && current != "healthy" {
		return &healthcheckError{message: fmt.Sprintf("container not ready after %d seconds (%s)", timeout, current)}
	}
	return nil
}

// prepareAssets extracts the role's assets and merges them with the running
// version's, as Kamal::Cli::App::Assets does.
func (c *CLI) prepareAssets(s *sshexec.Session, role *configuration.Role) error {
	if !role.Assets() {
		return nil
	}
	app := c.kamal.App(role, s.Host)
	if err := s.Execute(cmd(app.ExtractAssets())); err != nil {
		return err
	}
	old, err := s.CaptureWithInfo(lenientCmd(app.CurrentRunningVersion()))
	if err != nil {
		return err
	}
	return s.Execute(cmd(app.SyncAssetVolumes(strings.TrimSpace(old))))
}

// writeSSLCertificates uploads a role's custom certificate and key.
func (c *CLI) writeSSLCertificates(s *sshexec.Session, role *configuration.Role) error {
	if !role.RunningProxy() || !role.Proxy().CustomSSLCertificate() {
		return nil
	}
	s.Log(sshexec.Info, "Writing SSL certificates for "+role.Name()+" on "+s.Host)
	if err := s.Execute(cmd(c.kamal.App(role, s.Host).CreateSSLDirectory())); err != nil {
		return err
	}
	return c.uploadPEMs(s, role.Proxy())
}

func (c *CLI) uploadPEMs(s *sshexec.Session, proxy *configuration.Proxy) error {
	cert, err := proxy.CertificatePEMContent()
	if err != nil {
		return err
	}
	key, err := proxy.PrivateKeyPEMContent()
	if err != nil {
		return err
	}
	for path, content := range map[string]string{proxy.HostTLSCert(): cert, proxy.HostTLSKey(): key} {
		if content == "" {
			continue
		}
		if err := s.Upload(sshexec.Upload{Content: []byte(content), InMemory: true, Mode: "0644"}, path); err != nil {
			return err
		}
	}
	return nil
}

// uploadErrorPages uploads the 4xx and 5xx pages of error_pages_path, as
// Kamal::Cli::App::ErrorPages does.
func (c *CLI) uploadErrorPages(s *sshexec.Session) error {
	dir := c.config().ErrorPagesPath()
	if !dir.Truthy() {
		return nil
	}
	pages, err := c.errorPageFiles(dir.ToS())
	if err != nil || len(pages) == 0 {
		return err
	}
	if err := s.Execute(cmd(c.kamal.App(nil, "").CreateErrorPagesDirectory())); err != nil {
		return err
	}
	remote := filepath.Join(c.config().ProxyBoot().ErrorPagesDirectory(), c.config().Version())
	return c.uploadPages(s, pages, remote)
}

func (c *CLI) errorPageFiles(dir string) ([]string, error) {
	var pages []string
	for _, pattern := range []string{"4??.html", "5??.html"} {
		matches, err := c.rt.Files.Glob(filepath.Join(c.absolute(dir), pattern))
		if err != nil {
			return nil, err
		}
		pages = append(pages, matches...)
	}
	return pages, nil
}

func (c *CLI) uploadPages(s *sshexec.Session, pages []string, remote string) error {
	if err := s.Execute(cmd(words("mkdir", "-p", remote))); err != nil {
		return err
	}
	for _, page := range pages {
		if err := s.Upload(sshexec.Upload{LocalPath: page, Mode: "0700"}, filepath.Join(remote, filepath.Base(page))); err != nil {
			return err
		}
	}
	return nil
}
