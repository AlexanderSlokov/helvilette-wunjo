package cli

// Ports lib/kamal/cli/build.rb, build/clone.rb, and build/port_forwarding.rb at
// Kamal 2.12.0.

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/sshexec"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

type buildError struct{ message string }

func (e *buildError) Error() string { return e.message }

// RubyClass names the Kamal exception.
func (e *buildError) RubyClass() string { return "Kamal::Cli::Build::BuildError" }

func (c *CLI) buildSpec() spec {
	none := func(fn func(Options) error) func([]string, Options) error {
		return func(_ []string, o Options) error { return fn(o) }
	}
	output := func(def string) flag {
		return withDefault(stringFlag("output", "", "Exported type for the build result, and may be any exported type supported by 'buildx --output'."), def)
	}
	noCache := boolFlag("no_cache", "", "Build without using Docker's build cache")
	return spec{name: "build", desc: "Build application image", children: []spec{
		{name: "deliver", desc: "Build app and push app image to registry then pull image on servers", run: none(c.buildDeliver)},
		{name: "push", desc: "Build and push app image to registry", flags: []flag{output("registry"), noCache}, run: none(c.buildPush)},
		{name: "pull", desc: "Pull app image from registry onto servers", run: none(c.buildPull)},
		{name: "create", desc: "Create a build setup", run: none(c.buildCreate)},
		{name: "remove", desc: "Remove build setup", run: none(c.buildRemove)},
		{name: "details", desc: "Show build setup", run: none(c.buildDetails)},
		{name: "dev", desc: "Build using the working directory, tag it as dirty, and push to local image store.", flags: []flag{output("docker"), noCache}, run: none(c.buildDev)},
	}}
}

func (c *CLI) buildDeliver(opts Options) error {
	if err := c.invoke("build:push", nil, opts); err != nil {
		return err
	}
	return c.invoke("build:pull", nil, opts)
}

var missingBuilder = regexp.MustCompile(`(context not found|no builder|no compatible builder|does not exist)`)
var removableBuilder = regexp.MustCompile(`(context not found|no builder|does not exist)`)

func (c *CLI) buildPush(opts Options) error {
	if err := c.preConnectIfRequired(opts); err != nil {
		return err
	}
	if err := c.prepareBuild(opts); err != nil {
		return err
	}
	return c.forwardRegistryPortForRemoteBuilder(func() error {
		secrets, err := c.config().Builder().Secrets()
		if err != nil {
			return err
		}
		return c.runLocally(orderedEnv(secrets), func(s *sshexec.Session) error { return c.buildAndPush(s, opts) })
	})
}

func (c *CLI) prepareBuild(opts Options) error {
	if err := c.ensureDockerInstalled(); err != nil {
		return err
	}
	if err := c.loginForBuild(); err != nil {
		return err
	}
	if err := c.runHook(opts, "pre-build", false); err != nil {
		return err
	}
	return c.prepareBuildDirectory()
}

func (c *CLI) loginForBuild() error {
	if c.kamal.Registry().Local() {
		return c.runLocally(nil, func(s *sshexec.Session) error { return s.Execute(cmd(c.kamal.Registry().Setup(nil))) })
	}
	if !c.kamal.Builder().Target().LoginToRegistryLocally() {
		return nil
	}
	return c.runLocally(nil, c.executeLogin)
}

func (c *CLI) prepareBuildDirectory() error {
	changes, err := c.kamal.Git().UncommittedChanges()
	if err != nil {
		return err
	}
	if !c.config().Builder().GitClone() {
		if changes != "" {
			c.say("Building with uncommitted changes:\n "+changes, "yellow")
		}
		return nil
	}
	if changes != "" {
		c.say("Building from a local git clone, so ignoring these uncommitted changes:\n "+changes, "yellow")
	}
	return c.runLocally(nil, c.prepareClone)
}

func (c *CLI) buildAndPush(s *sshexec.Session, opts Options) error {
	if err := c.ensureBuilder(s, opts); err != nil {
		return err
	}
	target := c.kamal.Builder().Target()
	push, err := target.Push(opts.String("output"), false, opts.Bool("no_cache"))
	if err != nil {
		return err
	}
	dir, err := c.config().Builder().BuildDirectory()
	if err != nil {
		return err
	}
	command := cmd(push)
	command.Env, command.Dir = attrEnv(target.PushEnv()), c.absolute(dir)
	return c.kamal.WithVerbosity(sshexec.Debug, func() error { return s.Execute(command) })
}

// ensureBuilder creates the builder when inspecting it shows none.
func (c *CLI) ensureBuilder(s *sshexec.Session, opts Options) error {
	inspect, err := c.kamal.Builder().Target().InspectBuilder()
	if err != nil || inspect == nil {
		return err
	}
	err = s.Execute(cmd(inspect))
	if err == nil || !missingBuilder.MatchString(err.Error()) {
		return err
	}
	s.Log(sshexec.Warn, "Missing compatible builder, so creating a new one first")
	if removeErr := c.buildRemove(opts); removeErr != nil && !removableBuilder.MatchString(err.Error()) {
		return removeErr
	}
	return c.buildCreate(opts)
}

func (c *CLI) buildPull(opts Options) error {
	if !c.kamal.Registry().Local() {
		if err := c.registryLoginOn(opts, c.kamal.AppHosts()); err != nil {
			return err
		}
	}
	return c.forwardRegistryPort(c.kamal.Hosts(), c.sshOptions(), func() error { return c.pullWithMirrors(opts) })
}

func (c *CLI) pullWithMirrors(opts Options) error {
	first, err := c.mirrorHosts(opts)
	if err != nil {
		return err
	}
	if len(first) == 0 {
		return c.pullOnHosts(opts, c.kamal.AppHosts())
	}
	noun := "mirror"
	if len(first) > 1 {
		noun = "mirrors"
	}
	c.say("Pulling image on "+strings.Join(first, ", ")+" to seed the "+noun+"...", "magenta")
	if err := c.pullOnHosts(opts, first); err != nil {
		return err
	}
	c.say("Pulling image on remaining hosts...", "magenta")
	return c.pullOnHosts(opts, subtractHosts(c.kamal.AppHosts(), first))
}

var noMirror = regexp.MustCompile(`error calling index: reflect: slice index out of range`)

// mirrorHosts returns one host per registry mirror, to pull first and seed it.
func (c *CLI) mirrorHosts(opts Options) ([]string, error) {
	if len(c.kamal.AppHosts()) < 2 {
		return nil, nil
	}
	var mu sync.Mutex
	byMirror, order := map[string]string{}, []string{}
	err := c.on(opts, c.kamal.AppHosts(), func(s *sshexec.Session) error {
		mirror, err := s.CaptureWithInfo(cmd(c.kamal.Builder().FirstMirror()))
		if err != nil {
			if noMirror.MatchString(err.Error()) {
				return nil
			}
			return err
		}
		mu.Lock()
		defer mu.Unlock()
		if mirror != "" && byMirror[mirror] == "" {
			byMirror[mirror], order = s.Host, append(order, mirror)
		}
		return nil
	})
	var hosts []string
	for _, mirror := range order {
		hosts = append(hosts, byMirror[mirror])
	}
	return hosts, err
}

func (c *CLI) pullOnHosts(opts Options, hosts []string) error {
	builder := c.kamal.Builder()
	return c.on(opts, hosts, func(s *sshexec.Session) error {
		steps := []sshexec.Command{
			debugCmd(c.kamal.Auditor().Record("Pulled image with version " + c.config().Version())),
			lenientCmd(builder.Clean()), cmd(builder.Pull()), cmd(builder.ValidateImage()),
		}
		for _, step := range steps {
			if err := s.Execute(step); err != nil {
				return err
			}
		}
		return nil
	})
}

var stderrPattern = regexp.MustCompile(`stderr=(.*)`)

func (c *CLI) buildCreate(opts Options) error {
	if remote := c.config().Builder().Remote(); remote != "" {
		if err := c.connectToRemoteHost(opts, remote); err != nil {
			return err
		}
	}
	return c.runLocally(nil, func(s *sshexec.Session) error {
		target := c.kamal.Builder().Target()
		s.Log(sshexec.Debug, "Using builder: "+target.Name())
		create, err := target.Create()
		if err != nil || create == nil {
			return err
		}
		err = s.Execute(cmd(create))
		if m := stderrPatternMatch(err); m != "" {
			s.Log(sshexec.Error, "Couldn't create remote builder: "+m)
			return nil
		}
		return err
	})
}

func stderrPatternMatch(err error) string {
	if err == nil {
		return ""
	}
	if m := stderrPattern.FindStringSubmatch(err.Error()); m != nil {
		return m[1]
	}
	return ""
}

// connectToRemoteHost checks the SSH connection to an ssh:// remote builder.
func (c *CLI) connectToRemoteHost(opts Options, remote string) error {
	u, err := url.Parse(remote)
	if err != nil || u.Scheme != "ssh" {
		return nil
	}
	return c.on(opts, []string{u.Host}, func(s *sshexec.Session) error { return s.Execute(cmd(words("true"))) })
}

func (c *CLI) buildRemove(_ Options) error {
	return c.runLocally(nil, func(s *sshexec.Session) error {
		target := c.kamal.Builder().Target()
		s.Log(sshexec.Debug, "Using builder: "+target.Name())
		remove, err := target.Remove()
		if err != nil || remove == nil {
			return err
		}
		return s.Execute(cmd(remove))
	})
}

func (c *CLI) buildDetails(_ Options) error {
	return c.runLocally(nil, func(s *sshexec.Session) error {
		target := c.kamal.Builder().Target()
		c.puts("Builder: " + target.Name())
		output, err := s.Capture(cmd(target.Info()))
		c.puts(output)
		return err
	})
}

func (c *CLI) buildDev(opts Options) error {
	if err := c.ensureDockerInstalled(); err != nil {
		return err
	}
	secrets, err := c.config().Builder().Secrets()
	if err != nil {
		return err
	}
	return c.runLocally(orderedEnv(secrets), func(s *sshexec.Session) error {
		build, err := c.kamal.Builder().Target().Push(opts.String("output"), true, opts.Bool("no_cache"))
		if err != nil {
			return err
		}
		return c.kamal.WithVerbosity(sshexec.Debug, func() error { return s.Execute(cmd(build)) })
	})
}

// prepareClone clones the repository into the build directory, or resets an
// existing clone, and starts over once when the clone is broken.
func (c *CLI) prepareClone(s *sshexec.Session) error {
	err := c.cloneOrReset(s)
	if err == nil {
		err = c.validateClone(s)
	}
	var broken *buildError
	if !errors.As(err, &broken) {
		return err
	}
	s.Log(sshexec.Error, "Error preparing clone: "+broken.message+", deleting and retrying...")
	if err := c.rt.Files.RemoveAll(c.config().Builder().CloneDirectory()); err != nil {
		return err
	}
	if err := c.cloneRepo(s); err != nil {
		return err
	}
	return c.validateClone(s)
}

func (c *CLI) cloneOrReset(s *sshexec.Session) error {
	err := c.cloneRepo(s)
	var failed *sshexec.FailedError
	if !errors.As(err, &failed) {
		return err
	}
	if strings.Contains(err.Error(), "already exists and is not an empty directory") {
		return c.resetClone(s)
	}
	return &buildError{message: "Failed to clone repo: " + err.Error()}
}

func (c *CLI) cloneRepo(s *sshexec.Session) error {
	dir, err := c.config().Builder().BuildDirectory()
	if err != nil {
		return err
	}
	s.Log(sshexec.Info, "Cloning repo into build directory `"+dir+"`...")
	if err := c.rt.Files.MkdirAll(c.config().Builder().CloneDirectory()); err != nil {
		return err
	}
	clone, err := c.kamal.Builder().Clone()
	if err != nil {
		return err
	}
	return s.Execute(cmd(clone))
}

func (c *CLI) resetClone(s *sshexec.Session) error {
	dir, _ := c.config().Builder().BuildDirectory()
	s.Log(sshexec.Info, "Resetting local clone as `"+dir+"` already exists...")
	steps, err := c.kamal.Builder().CloneResetSteps()
	if err != nil {
		return err
	}
	for _, step := range steps {
		if err := s.Execute(cmd(step)); err != nil {
			return &buildError{message: "Failed to clone repo: " + err.Error()}
		}
	}
	return nil
}

func (c *CLI) validateClone(s *sshexec.Session) error {
	dir, _ := c.config().Builder().BuildDirectory()
	status, revision, err := c.cloneState(s)
	if err != nil {
		return &buildError{message: "Failed to validate clone: " + err.Error()}
	}
	if status != "" {
		return &buildError{message: "Clone in " + dir + " is dirty, " + status}
	}
	expected, err := c.kamal.Git().Revision()
	if err != nil {
		return err
	}
	if revision != expected {
		return &buildError{message: "Clone in " + dir + " is not on the correct revision, expected `" + expected + "` but got `" + revision + "`"}
	}
	return nil
}

func (c *CLI) cloneState(s *sshexec.Session) (string, string, error) {
	statusLine, err := c.kamal.Builder().CloneStatus()
	if err != nil {
		return "", "", err
	}
	status, err := s.CaptureWithInfo(cmd(statusLine))
	if err != nil {
		return "", "", err
	}
	revisionLine, err := c.kamal.Builder().CloneRevision()
	if err != nil {
		return "", "", err
	}
	revision, err := s.CaptureWithInfo(cmd(revisionLine))
	return status, revision, err
}

// forwardRegistryPortForRemoteBuilder forwards a local registry to a remote builder.
func (c *CLI) forwardRegistryPortForRemoteBuilder(fn func() error) error {
	if !c.config().Builder().IsRemote() {
		return fn()
	}
	u, err := url.Parse(c.config().Builder().Remote())
	if err != nil {
		return err
	}
	ssh := c.sshOptions()
	ssh.User, ssh.Port = u.User.Username(), u.Port()
	return c.forwardRegistryPort([]string{u.Hostname()}, ssh, fn)
}

// forwardRegistryPort opens remote port forwards to a registry on localhost while fn
// runs.
func (c *CLI) forwardRegistryPort(hosts []string, ssh sshexec.Options, fn func() error) error {
	if !c.config().Registry().Local() {
		return fn()
	}
	c.say("Setting up local registry port forwarding to "+strings.Join(hosts, ", ")+"...", "")
	port, _ := c.config().Registry().LocalPort()
	stop, err := c.rt.Forward(hosts, port, ssh)
	if err != nil {
		return err
	}
	defer stop()
	return fn()
}

// sshOptions maps the ssh section of the deploy file to sshexec options.
func (c *CLI) sshOptions() sshexec.Options {
	return SSHOptions(c.config())
}

// SSHOptions maps the ssh and sshkit sections of cfg to sshexec options.
func SSHOptions(cfg *configuration.Configuration) sshexec.Options {
	ssh := cfg.SSH()
	opts := sshexec.Options{User: ssh.User(), Port: ssh.Port(), Keys: ssh.Keys().StringList(), KeysOnly: ssh.KeysOnly().Truthy(),
		LogLevel: ssh.LogLevel(), ControlPersist: cfg.Sshkit().PoolIdleTimeout()}
	if proxy, ok := ssh.Proxy(); ok {
		opts.ProxyJump, opts.ProxyCommand = proxy.Jump, proxy.Command
	}
	opts.KeyData, _, _ = ssh.KeyData()
	applySSHConfig(&opts, ssh.Config())
	if agent := ssh.ForwardAgent(); !agent.IsNull() {
		forward := agent.Truthy()
		opts.ForwardAgent = &forward
	}
	return opts
}

func applySSHConfig(opts *sshexec.Options, config interface {
	IsFalse() bool
	IsTrue() bool
	IsNull() bool
	StringList() []string
}) {
	switch {
	case config.IsFalse():
		opts.IgnoreConfig = true
	case config.IsTrue() || config.IsNull():
	default:
		opts.ConfigFiles = config.StringList()
	}
}

// orderedEnv turns build secrets into process variables, as with_env does.
func orderedEnv(m *yamlord.Ordered) []sshexec.EnvVar {
	var env []sshexec.EnvVar
	m.Each(func(key string, value *yamlord.Value) {
		env = append(env, sshexec.EnvVar{Key: key, Value: value.ToS()})
	})
	return env
}

// attrEnv turns a builder's push env into command variables.
func attrEnv(attrs []utils.Attr) []sshexec.EnvVar {
	env := make([]sshexec.EnvVar, len(attrs))
	for i, attr := range attrs {
		env[i] = sshexec.EnvVar{Key: attr.Key, Value: attr.Value.ToS()}
	}
	return env
}
