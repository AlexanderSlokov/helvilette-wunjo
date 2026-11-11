package cli

// Ports lib/kamal/cli/app.rb at Kamal 2.12.0.

import (
	"sort"
	"strings"
	"time"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/commands"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/sshexec"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/tags"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

var logFlags = []flag{
	stringFlag("since", "s", "Show lines since timestamp (e.g. 2013-01-02T13:23:37Z) or relative (e.g. 42m for 42 minutes)"),
	numberFlag("lines", "n", "Number of lines to show from each server"),
	stringFlag("grep", "g", "Show lines with grep match only (use this to fetch specific requests by id)"),
	stringFlag("grep_options", "", "Additional options supplied to grep"),
	optionalBool("follow", "f", "Follow log on primary server (or specific host set by --hosts)"),
	optionalBool("skip_timestamps", "T", "Skip appending timestamps to logging output"),
}

func (c *CLI) appSpec() spec {
	none := func(fn func(Options) error) func([]string, Options) error {
		return func(_ []string, o Options) error { return fn(o) }
	}
	return spec{name: "app", desc: "Manage application", children: []spec{
		{name: "boot", desc: "Boot app on servers (or reboot app if already running)", run: none(c.appBoot)},
		{name: "start", desc: "Start existing app container on servers", run: none(c.appStart)},
		{name: "stop", desc: "Stop app container on servers", run: none(c.appStop)},
		{name: "details", desc: "Show details about app containers", run: none(c.appDetails)},
		{name: "exec", args: "[CMD...]", desc: "Execute a custom command on servers within the app container (use --help to show options)", maxArgs: anyArgs, flags: appExecFlags, run: c.appExec},
		{name: "containers", desc: "Show app containers on servers", run: none(c.appContainers)},
		{name: "stale_containers", desc: "Detect app stale containers", flags: []flag{boolFlag("stop", "s", "Stop the stale containers found")}, run: none(c.appStaleContainers)},
		{name: "images", desc: "Show app images on servers", run: none(c.appImages)},
		{name: "logs", desc: "Show log lines from app on servers (use --help to show options)", flags: append(append([]flag{}, logFlags...), stringFlag("container_id", "", "Docker container ID to fetch logs")), run: none(c.appLogs)},
		{name: "remove", desc: "Remove app containers and images from servers", run: none(c.appRemove)},
		{name: "live", desc: "Set the app to live mode", run: none(c.appLive)},
		{name: "maintenance", desc: "Set the app to maintenance mode", flags: []flag{numberFlag("drain_timeout", "", "How long to allow in-flight requests to complete (defaults to drain_timeout from config)"), stringFlag("message", "", "Message to display to clients while stopped")}, run: none(c.appMaintenance)},
		{name: "remove_container", args: "[VERSION]", desc: "Remove app container with given version from servers", hidden: true, minArgs: 1, maxArgs: 1, run: c.appRemoveContainer},
		{name: "remove_containers", desc: "Remove all app containers from servers", hidden: true, run: none(c.appRemoveContainers)},
		{name: "remove_images", desc: "Remove all app images from servers", hidden: true, run: none(c.appRemoveImages)},
		{name: "remove_app_directories", desc: "Remove the app directories from servers", hidden: true, run: none(c.appRemoveAppDirectories)},
		{name: "version", desc: "Show app version currently running on servers", run: none(c.appVersion)},
	}}
}

const anyArgs = -1

var appExecFlags = []flag{
	boolFlag("interactive", "i", "Execute command over ssh for an interactive shell (use for console/bash)"),
	boolFlag("reuse", "", "Reuse currently running container instead of starting a new one"),
	hash(stringFlag("env", "e", "Set environment variables for the command")),
	boolFlag("detach", "", "Execute command in a detached container"),
	boolFlag("raw", "", "Output raw, unmodified stdout"),
}

func (c *CLI) parallelRoles() bool { return c.config().Boot().ParallelRoles().Truthy() }

func (c *CLI) appBoot(opts Options) error {
	return c.modify(true, func() error {
		if !opts.Has("version") {
			c.say("Get most recent version available as an image...", "magenta")
		}
		return c.usingVersion(c.versionOrLatest(opts), func(version string) error {
			c.say("Start container with version "+version+" (or reboot if already running)...", "magenta")
			if err := c.prepareBoot(opts); err != nil {
				return err
			}
			if err := c.bootGroups(opts, version); err != nil {
				return err
			}
			return c.tagLatest(opts)
		})
	})
}

// prepareBoot uploads error pages, assets, and certificates to every host first, so
// they are in place before any container starts.
func (c *CLI) prepareBoot(opts Options) error {
	return c.on(opts, c.kamal.AppHosts(), func(s *sshexec.Session) error {
		if err := c.uploadErrorPages(s); err != nil {
			return err
		}
		for _, role := range c.kamal.RolesOn(s.Host) {
			if err := c.prepareAssets(s, role); err != nil {
				return err
			}
			if err := c.writeSSLCertificates(s, role); err != nil {
				return err
			}
		}
		return nil
	})
}

// bootGroups boots the hosts in groups of boot/limit. The primary hosts come first,
// so they can open the barrier for the other roles.
func (c *CLI) bootGroups(opts Options, version string) error {
	barrier := newBarrier()
	for _, hosts := range c.hostBootGroups() {
		list := strings.Join(hosts, ",")
		if err := c.runHook(opts, "pre-app-boot", false, tags.Tag{Name: "hosts", Value: list}); err != nil {
			return err
		}
		err := c.onRoles(opts, c.kamal.Roles(), hosts, c.parallelRoles(), func(s *sshexec.Session, role *configuration.Role) error {
			return c.newBoot(s, role, version, barrier).run()
		})
		if err != nil {
			return err
		}
		if err := c.runHook(opts, "post-app-boot", false, tags.Tag{Name: "hosts", Value: list}); err != nil {
			return err
		}
		c.sleepBootWait()
	}
	return nil
}

func (c *CLI) sleepBootWait() {
	if wait := c.config().Boot().Wait(); wait.Truthy() {
		c.rt.Sleep(time.Duration(wait.Float() * float64(time.Second)))
	}
}

func (c *CLI) tagLatest(opts Options) error {
	return c.on(opts, c.kamal.AppHosts(), func(s *sshexec.Session) error {
		if err := s.Execute(debugCmd(c.kamal.Auditor().Record("Tagging " + c.config().AbsoluteImage() + " as the latest image"))); err != nil {
			return err
		}
		return s.Execute(cmd(c.kamal.App(nil, "").TagLatestImage()))
	})
}

func (c *CLI) hostBootGroups() [][]string {
	hosts := c.kamal.AppHosts()
	limit := c.config().Boot().Limit()
	if !limit.Truthy() {
		return [][]string{hosts}
	}
	size := int(limit.Int())
	var groups [][]string
	for start := 0; start < len(hosts); start += size {
		groups = append(groups, hosts[start:min(start+size, len(hosts))])
	}
	return groups
}

func (c *CLI) appStart(opts Options) error {
	return c.modify(true, func() error {
		return c.onRoles(opts, c.kamal.Roles(), c.kamal.AppHosts(), c.parallelRoles(), func(s *sshexec.Session, role *configuration.Role) error {
			app := c.kamal.App(role, s.Host)
			if err := s.Execute(debugCmd(c.kamal.Auditor().Record("Started app version " + c.config().Version()))); err != nil {
				return err
			}
			if err := s.Execute(lenientCmd(app.Start())); err != nil {
				return err
			}
			if !role.RunningProxy() {
				return nil
			}
			return c.deployRunningContainer(s, app, role)
		})
	})
}

// deployRunningContainer points kamal-proxy at the running container of the role.
func (c *CLI) deployRunningContainer(s *sshexec.Session, app *commands.App, role *configuration.Role) error {
	version, err := s.CaptureWithInfo(lenientCmd(app.CurrentRunningVersion()))
	if err != nil {
		return err
	}
	endpoint, err := s.CaptureWithInfo(cmd(app.ContainerIDForVersion(strings.TrimSpace(version), false)))
	if err != nil {
		return err
	}
	if endpoint == "" {
		return bootError("Failed to get endpoint for " + role.Name() + " on " + s.Host + ", did the container boot?")
	}
	return s.Execute(cmd(app.Deploy(endpoint)))
}

func (c *CLI) appStop(opts Options) error {
	return c.modify(true, func() error {
		return c.onRoles(opts, c.kamal.Roles(), c.kamal.AppHosts(), c.parallelRoles(), func(s *sshexec.Session, role *configuration.Role) error {
			app := c.kamal.App(role, s.Host)
			if err := s.Execute(debugCmd(c.kamal.Auditor(tags.Tag{Name: "role", Value: role.Name()}).Record("Stopped app"))); err != nil {
				return err
			}
			if err := c.removeFromProxy(s, app, role); err != nil {
				return err
			}
			return s.Execute(lenientCmd(app.Stop("")))
		})
	})
}

func (c *CLI) removeFromProxy(s *sshexec.Session, app *commands.App, role *configuration.Role) error {
	if !role.RunningProxy() {
		return nil
	}
	version, err := s.CaptureWithInfo(lenientCmd(app.CurrentRunningVersion()))
	if err != nil {
		return err
	}
	endpoint, err := s.CaptureWithInfo(cmd(app.ContainerIDForVersion(strings.TrimSpace(version), false)))
	if err != nil || endpoint == "" {
		return err
	}
	return s.Execute(lenientCmd(app.Remove()))
}

func (c *CLI) appDetails(opts Options) error {
	return c.onRoles(opts, c.kamal.Roles(), c.kamal.AppHosts(), true, func(s *sshexec.Session, role *configuration.Role) error {
		output, err := s.CaptureWithInfo(cmd(c.kamal.App(role, s.Host).Info()))
		c.putsByHost(s.Host, output, "App", opts.Bool("quiet"), false)
		return err
	})
}

func (c *CLI) appContainers(opts Options) error {
	return c.on(opts, c.kamal.AppHosts(), func(s *sshexec.Session) error {
		output, err := s.CaptureWithInfo(cmd(c.kamal.App(nil, "").ListContainers()))
		c.putsByHost(s.Host, output, "App", opts.Bool("quiet"), false)
		return err
	})
}

func (c *CLI) appStaleContainers(opts Options) error {
	return c.withLockIfStopping(opts, func() error {
		return c.onRoles(opts, c.kamal.Roles(), c.kamal.AppHosts(), true, func(s *sshexec.Session, role *configuration.Role) error {
			return c.handleStaleContainers(s, role, opts)
		})
	})
}

func (c *CLI) handleStaleContainers(s *sshexec.Session, role *configuration.Role, opts Options) error {
	app := c.kamal.App(role, s.Host)
	versions, err := c.staleVersions(s, app)
	if err != nil {
		return err
	}
	for _, version := range versions {
		if !opts.Bool("stop") {
			c.putsByHost(s.Host, "Detected stale container for role "+role.Name()+" with version "+version+" (use `kamal app stale_containers --stop` to stop)", "App", opts.Bool("quiet"), false)
			continue
		}
		c.putsByHost(s.Host, "Stopping stale container for role "+role.Name()+" with version "+version, "App", opts.Bool("quiet"), false)
		if err := s.Execute(lenientCmd(app.Stop(version))); err != nil {
			return err
		}
	}
	return nil
}

func (c *CLI) staleVersions(s *sshexec.Session, app *commands.App) ([]string, error) {
	listed, err := s.CaptureWithInfo(lenientCmd(app.ListVersions(nil, nil)))
	if err != nil {
		return nil, err
	}
	current, err := s.CaptureWithInfo(lenientCmd(app.CurrentRunningVersion()))
	if err != nil {
		return nil, err
	}
	var versions []string
	for _, version := range strings.Split(listed, "\n") {
		if version != "" && version != strings.TrimSpace(current) {
			versions = append(versions, version)
		}
	}
	return versions, nil
}

func (c *CLI) withLockIfStopping(opts Options, fn func() error) error {
	if opts.Bool("stop") {
		return c.modify(true, fn)
	}
	return fn()
}

func (c *CLI) appImages(opts Options) error {
	return c.on(opts, c.kamal.AppHosts(), func(s *sshexec.Session) error {
		output, err := s.CaptureWithInfo(cmd(c.kamal.App(nil, "").ListImages()))
		c.putsByHost(s.Host, output, "App", opts.Bool("quiet"), false)
		return err
	})
}

func (c *CLI) appRemove(opts Options) error {
	return c.modify(true, func() error {
		steps := []func() error{
			func() error { return c.runHook(opts, "pre-app-remove", false) },
			func() error { return c.appStop(opts) }, func() error { return c.appRemoveContainers(opts) },
			func() error { return c.appRemoveImages(opts) }, func() error { return c.appRemoveAppDirectories(opts) },
			func() error { return c.runHook(opts, "post-app-remove", false) },
		}
		return runSteps(steps)
	})
}

func runSteps(steps []func() error) error {
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

func (c *CLI) appLive(opts Options) error {
	return c.modify(true, func() error {
		return c.onRoles(opts, c.kamal.Roles(), c.kamal.ProxyHosts(), true, func(s *sshexec.Session, role *configuration.Role) error {
			if !role.RunningProxy() {
				return nil
			}
			return s.Execute(cmd(c.kamal.App(role, s.Host).Live()))
		})
	})
}

func (c *CLI) appMaintenance(opts Options) error {
	drain := c.config().DrainTimeout()
	if timeout, ok := opts.Int("drain_timeout"); ok {
		drain = yamlord.NewInt(timeout)
	}
	return c.modify(true, func() error {
		return c.onRoles(opts, c.kamal.Roles(), c.kamal.ProxyHosts(), true, func(s *sshexec.Session, role *configuration.Role) error {
			if !role.RunningProxy() {
				return nil
			}
			return s.Execute(cmd(c.kamal.App(role, s.Host).Maintenance(drain, opts.String("message"))))
		})
	})
}

func (c *CLI) appRemoveContainer(args []string, opts Options) error {
	version := args[0]
	return c.modify(true, func() error {
		return c.onRoles(opts, c.kamal.Roles(), c.kamal.AppHosts(), true, func(s *sshexec.Session, role *configuration.Role) error {
			if err := s.Execute(debugCmd(c.kamal.Auditor(tags.Tag{Name: "role", Value: role.Name()}).Record("Removed app container with version " + version))); err != nil {
				return err
			}
			return s.Execute(cmd(c.kamal.App(role, s.Host).RemoveContainer(version)))
		})
	})
}

func (c *CLI) appRemoveContainers(opts Options) error {
	return c.modify(true, func() error {
		return c.onRoles(opts, c.kamal.Roles(), c.kamal.AppHosts(), true, func(s *sshexec.Session, role *configuration.Role) error {
			if err := s.Execute(debugCmd(c.kamal.Auditor(tags.Tag{Name: "role", Value: role.Name()}).Record("Removed all app containers"))); err != nil {
				return err
			}
			return s.Execute(cmd(c.kamal.App(role, s.Host).RemoveContainers()))
		})
	})
}

func (c *CLI) appRemoveImages(opts Options) error {
	return c.modify(true, func() error {
		return c.on(opts, c.hostsRemovingAllRoles(), func(s *sshexec.Session) error {
			if err := s.Execute(debugCmd(c.kamal.Auditor().Record("Removed all app images"))); err != nil {
				return err
			}
			return s.Execute(cmd(c.kamal.App(nil, "").RemoveImages()))
		})
	})
}

func (c *CLI) appRemoveAppDirectories(opts Options) error {
	return c.modify(true, func() error {
		return c.on(opts, c.hostsRemovingAllRoles(), func(s *sshexec.Session) error {
			if err := s.Execute(lenientCmd(c.kamal.Server().RemoveAppDirectory())); err != nil {
				return err
			}
			if err := s.Execute(debugCmd(c.kamal.Auditor().Record("Removed " + c.config().AppDirectory()))); err != nil {
				return err
			}
			return s.Execute(lenientCmd(c.kamal.App(nil, "").RemoveProxyAppDirectory()))
		})
	})
}

func (c *CLI) appVersion(opts Options) error {
	return c.on(opts, c.kamal.AppHosts(), func(s *sshexec.Session) error {
		role := c.kamal.RolesOn(s.Host)[0]
		output, err := s.CaptureWithInfo(cmd(c.kamal.App(role, s.Host).CurrentRunningVersion()))
		c.putsByHost(s.Host, strings.TrimSpace(output), "App", opts.Bool("quiet"), false)
		return err
	})
}

// hostsRemovingAllRoles returns the app hosts where every role of the host is
// selected, so removing images there leaves no role without one.
func (c *CLI) hostsRemovingAllRoles() []string {
	var hosts []string
	for _, host := range c.kamal.AppHosts() {
		if sameNames(c.kamal.RolesOn(host), c.config().HostRoles(host)) {
			hosts = append(hosts, host)
		}
	}
	return hosts
}

func sameNames(a, b []*configuration.Role) bool {
	names := func(roles []*configuration.Role) string {
		var out []string
		for _, role := range roles {
			out = append(out, role.Name())
		}
		sort.Strings(out)
		return strings.Join(out, ",")
	}
	return names(a) == names(b)
}

// usingVersion runs fn with the configuration set to version, and restores the
// previous version after it.
func (c *CLI) usingVersion(version string, fn func(string) error) error {
	if version == "" {
		return fn(c.config().Version())
	}
	previous := c.config().Version()
	c.config().SetVersion(version)
	defer c.config().SetVersion(previous)
	return fn(version)
}

func (c *CLI) versionOrLatest(opts Options) string {
	if version := opts.String("version"); version != "" {
		return version
	}
	return c.config().LatestTag()
}

// currentRunningVersion returns the version that runs on host, or "".
func (c *CLI) currentRunningVersion(opts Options, host string) (string, error) {
	var version string
	err := c.on(opts, []string{host}, func(s *sshexec.Session) error {
		role := c.kamal.RolesOn(host)[0]
		out, err := s.CaptureWithInfo(cmd(c.kamal.App(role, host).CurrentRunningVersion()))
		version = strings.TrimSpace(out)
		return err
	})
	return version, err
}

var errNoCommand = argumentError("No command provided. You must specify a command to execute.")

// execEnv reads the -e KEY:VALUE pairs, which cliframe has checked as Thor's
// hash option does.
func execEnv(opts Options) *yamlord.Ordered {
	env := yamlord.NewOrdered()
	for _, pair := range opts.List("env") {
		key, value, _ := strings.Cut(pair, ":")
		env.Set(key, yamlord.NewString(value))
	}
	return env
}

func checkExecOptions(opts Options, cmdArgs []string) error {
	var incompatible []string
	for _, key := range []string{"interactive", "reuse"} {
		if opts.Bool("detach") && opts.Bool(key) {
			incompatible = append(incompatible, key)
		}
	}
	if len(incompatible) > 0 {
		return argumentError("Detach is not compatible with " + strings.Join(incompatible, " or "))
	}
	if err := checkRawOptions(opts, "interactive", "detach"); err != nil {
		return err
	}
	if len(cmdArgs) == 0 {
		return errNoCommand
	}
	return nil
}

func checkRawOptions(opts Options, keys ...string) error {
	if !opts.Bool("raw") {
		return nil
	}
	var incompatible []string
	for _, key := range keys {
		if opts.Bool(key) {
			incompatible = append(incompatible, key)
		}
	}
	if len(incompatible) > 0 {
		return argumentError("Raw is not compatible with " + strings.Join(incompatible, " or "))
	}
	return nil
}
