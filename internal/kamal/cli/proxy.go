package cli

// Ports lib/kamal/cli/proxy.rb at Kamal 2.12.0.

import (
	"errors"
	"strconv"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/commands"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/sshexec"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/tags"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

func (c *CLI) proxySpec() spec {
	none := func(fn func(Options) error) func([]string, Options) error {
		return func(_ []string, o Options) error { return fn(o) }
	}
	rolling := boolFlag("rolling", "", "Reboot proxy on hosts in sequence, rather than in parallel")
	return spec{name: "proxy", desc: "Manage kamal-proxy", children: []spec{
		{name: "boot", desc: "Boot proxy on servers", run: none(c.proxyBoot)},
		{name: "boot_config", args: "<set|get|reset>", desc: "Manage kamal-proxy boot configuration", minArgs: 1, maxArgs: 1, flags: bootConfigFlags, run: c.proxyBootConfig},
		{name: "reboot", desc: "Reboot proxy on servers (stop container, remove container, start new container)", flags: []flag{rolling, confirmedFlag}, run: none(c.proxyReboot)},
		{name: "upgrade", desc: "Upgrade to kamal-proxy on servers (stop container, remove container, start new container, reboot app)", hidden: true, flags: []flag{rolling, confirmedFlag}, run: none(c.proxyUpgrade)},
		{name: "start", desc: "Start existing proxy container on servers", run: none(c.proxyStart)},
		{name: "stop", desc: "Stop existing proxy container on servers", run: none(c.proxyStop)},
		{name: "restart", desc: "Restart existing proxy container on servers", run: none(c.proxyRestart)},
		{name: "details", desc: "Show details about proxy container from servers", run: none(c.proxyDetails)},
		{name: "logs", desc: "Show log lines from proxy on servers", flags: []flag{logFlags[0], logFlags[1], logFlags[2], logFlags[4], logFlags[5]}, run: none(c.proxyLogs)},
		{name: "remove", desc: "Remove proxy container and image from servers", flags: []flag{boolFlag("force", "", "Force removing proxy when apps are still installed")}, run: none(c.proxyRemove)},
		{name: "remove_container", desc: "Remove proxy container from servers", hidden: true, run: none(c.proxyRemoveContainer)},
		{name: "remove_image", desc: "Remove proxy image from servers", hidden: true, run: none(c.proxyRemoveImage)},
		{name: "remove_proxy_directory", desc: "Remove the proxy directory from servers", hidden: true, run: none(c.proxyRemoveProxyDirectory)},
	}}
}

var bootConfigFlags = []flag{
	withDefault(optionalBool("publish", "", "Publish the proxy ports on the host"), "true"),
	repeatable(stringFlag("publish_host_ip", "", "Host IP address to bind HTTP/HTTPS traffic to. Defaults to all interfaces")),
	withDefault(numberFlag("http_port", "", "HTTP port to publish on the host"), "80"),
	withDefault(numberFlag("https_port", "", "HTTPS port to publish on the host"), "443"),
	withDefault(stringFlag("log_max_size", "", "Max size of proxy logs"), "10m"),
	stringFlag("registry", "", "Registry to use for the proxy image"),
	stringFlag("repository", "", "Repository for the proxy image"),
	stringFlag("image_version", "", "Version of the proxy to run"),
	numberFlag("metrics_port", "", "Port to report prometheus metrics on"),
	boolFlag("debug", "", "Whether to run the proxy in debug mode"),
	array(stringFlag("docker_options", "", "Docker options to pass to the proxy container")),
}

func (c *CLI) proxyBoot(opts Options) error {
	return c.modify(true, func() error {
		if err := c.createNetworks(opts); err != nil {
			return err
		}
		return c.on(opts, c.kamal.ProxyHosts(), func(s *sshexec.Session) error { return c.bootProxyOn(s) })
	})
}

func (c *CLI) createNetworks(opts Options) error {
	return c.on(opts, c.kamal.Hosts(), func(s *sshexec.Session) error {
		err := s.Execute(cmd(c.kamal.Docker().CreateNetwork()))
		if err != nil && strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return err
	})
}

func (c *CLI) bootProxyOn(s *sshexec.Session) error {
	if err := c.executeLogin(s); err != nil {
		return err
	}
	proxy := c.kamal.Proxy(s.Host)
	version, err := s.CaptureWithInfo(cmd(proxy.Version()))
	if err != nil {
		return err
	}
	minimum := configuration.ProxyMinimumVersion()
	if version != "" && utils.OlderVersion(version, minimum) {
		return errors.New("kamal-proxy version " + version + " is too old, run `kamal proxy reboot` in order to update to at least " + minimum)
	}
	if err := s.Execute(cmd(proxy.EnsureAppsConfigDirectory())); err != nil {
		return err
	}
	return s.Execute(cmd(proxy.StartOrRun()))
}

func (c *CLI) proxyBootConfig(args []string, opts Options) error {
	c.say("The proxy boot_config command is deprecated - set the config in the deploy YAML at proxy/run instead", "yellow")
	switch args[0] {
	case "set":
		return c.setBootConfig(opts)
	case "get":
		return c.on(opts, c.kamal.ProxyHosts(), func(s *sshexec.Session) error {
			output, err := s.CaptureWithInfo(cmd(c.kamal.Proxy(s.Host).BootConfig()))
			c.puts("Host " + s.Host + ": " + output)
			return err
		})
	case "reset":
		return c.on(opts, c.kamal.ProxyHosts(), func(s *sshexec.Session) error { return c.resetBootConfig(s) })
	default:
		return argumentError("Unknown boot_config subcommand " + args[0])
	}
}

func (c *CLI) resetBootConfig(s *sshexec.Session) error {
	proxy := c.kamal.Proxy(s.Host)
	for _, line := range []shell.Line{proxy.ResetBootOptions(), proxy.ResetImage(), proxy.ResetImageVersion(), proxy.ResetRunCommand()} {
		if err := s.Execute(lenientCmd(line)); err != nil {
			return err
		}
	}
	return nil
}

// bootSettings is what `proxy boot_config set` writes to each host.
type bootSettings struct {
	options, image, imageVersion, runCommand string
	defaultOptions                           bool
}

func (c *CLI) bootSettingsFrom(opts Options) (bootSettings, error) {
	boot := c.config().ProxyBoot()
	options, err := c.bootOptions(opts)
	if err != nil {
		return bootSettings{}, err
	}
	image := strings.Join(nonEmptyStrings(opts.String("registry"), orDefault(opts.String("repository"), boot.RepositoryName()), boot.ImageName()), "/")
	return bootSettings{
		options: strings.Join(options, " "), defaultOptions: strings.Join(options, " ") == strings.Join(boot.DefaultBootOptions(), " "),
		image: image, imageVersion: opts.String("image_version"), runCommand: runCommandFrom(opts),
	}, nil
}

func (c *CLI) bootOptions(opts Options) ([]string, error) {
	boot := c.config().ProxyBoot()
	var options []string
	if opts.Bool("publish") {
		http, _ := opts.Int("http_port")
		https, _ := opts.Int("https_port")
		publish, err := boot.PublishArgs(yamlord.NewInt(http), yamlord.NewInt(https), opts.List("publish_host_ip"))
		if err != nil {
			return nil, argumentError(err.Error())
		}
		options = append(options, publish)
	}
	options = append(options, wordsText(boot.LoggingArgs(opts.String("log_max_size")))...)
	if port, ok := opts.Int("metrics_port"); ok {
		options = append(options, "--expose="+strconv.FormatInt(port, 10))
	}
	for _, option := range opts.List("docker_options") {
		options = append(options, "--"+option)
	}
	return options, nil
}

func runCommandFrom(opts Options) string {
	var attrs []utils.Attr
	if opts.Bool("debug") {
		attrs = append(attrs, utils.Attr{Key: "debug", Value: yamlord.NewBool(true)})
	}
	if port, ok := opts.Int("metrics_port"); ok {
		attrs = append(attrs, utils.Attr{Key: "metrics-port", Value: yamlord.NewInt(port)})
	}
	if len(attrs) == 0 {
		return ""
	}
	return "kamal-proxy run " + utils.Optionize(attrs).String()
}

func (c *CLI) setBootConfig(opts Options) error {
	settings, err := c.bootSettingsFrom(opts)
	if err != nil {
		return err
	}
	boot := c.config().ProxyBoot()
	return c.on(opts, c.kamal.ProxyHosts(), func(s *sshexec.Session) error {
		proxy := c.kamal.Proxy(s.Host)
		if err := s.Execute(cmd(proxy.EnsureProxyDirectory())); err != nil {
			return err
		}
		files := []struct {
			content, path string
			write         bool
			reset         shell.Line
		}{
			{settings.options, boot.OptionsFile(), !settings.defaultOptions, proxy.ResetBootOptions()},
			{settings.image, boot.ImageFile(), settings.image != boot.ImageDefault(), proxy.ResetImage()},
			{settings.imageVersion, boot.ImageVersionFile(), settings.imageVersion != "", proxy.ResetImageVersion()},
			{settings.runCommand, boot.RunCommandFile(), settings.runCommand != "", proxy.ResetRunCommand()},
		}
		for _, file := range files {
			if err := c.writeOrReset(s, file.write, file.content, file.path, file.reset); err != nil {
				return err
			}
		}
		return nil
	})
}

func (c *CLI) writeOrReset(s *sshexec.Session, write bool, content, path string, reset shell.Line) error {
	if write {
		return s.Upload(sshexec.Upload{Content: []byte(content), InMemory: true}, path)
	}
	return s.Execute(lenientCmd(reset))
}

func (c *CLI) proxyReboot(opts Options) error {
	return c.confirming(opts, "This will cause a brief outage on each host. Are you sure?", func() error {
		return c.modify(true, func() error {
			for _, hosts := range hostGroups(c.kamal.ProxyHosts(), opts.Bool("rolling")) {
				if err := c.rebootProxyOn(hosts, opts); err != nil {
					return err
				}
			}
			return nil
		})
	})
}

func hostGroups(hosts []string, rolling bool) [][]string {
	if !rolling {
		return [][]string{hosts}
	}
	groups := make([][]string, len(hosts))
	for i, host := range hosts {
		groups[i] = []string{host}
	}
	return groups
}

func (c *CLI) rebootProxyOn(hosts []string, opts Options) error {
	list := tags.Tag{Name: "hosts", Value: strings.Join(hosts, ",")}
	if err := c.runHook(opts, "pre-proxy-reboot", false, list); err != nil {
		return err
	}
	err := c.on(opts, hosts, func(s *sshexec.Session) error {
		proxy := c.kamal.Proxy(s.Host)
		login, err := c.loginCmds()
		if err != nil {
			return err
		}
		steps := append(append([]sshexec.Command{debugCmd(c.kamal.Auditor().Record("Rebooted proxy"))}, login...),
			lenientCmd(proxy.Stop("")), cmd(proxy.RemoveContainer()), cmd(proxy.EnsureAppsConfigDirectory()), cmd(proxy.Run()))
		return executeAll(s, steps...)
	})
	if err != nil {
		return err
	}
	return c.runHook(opts, "post-proxy-reboot", false, list)
}

// loginCmds returns docker login for the deploy registry, or none for a local one.
func (c *CLI) loginCmds() ([]sshexec.Command, error) {
	line, err := c.kamal.Registry().Login(nil)
	if err != nil || line == nil {
		return nil, err
	}
	return []sshexec.Command{cmd(line)}, nil
}

func executeAll(s *sshexec.Session, steps ...sshexec.Command) error {
	for _, step := range steps {
		if err := s.Execute(step); err != nil {
			return err
		}
	}
	return nil
}

func (c *CLI) proxyUpgrade(opts Options) error {
	invokeOptions := Options{"version": c.config().LatestTag()}.Merge(opts)
	return c.confirming(opts, "This will cause a brief outage on each host. Are you sure?", func() error {
		for _, hosts := range hostGroups(c.kamal.Hosts(), opts.Bool("rolling")) {
			if err := c.upgradeProxyOn(hosts, opts, invokeOptions); err != nil {
				return err
			}
		}
		return nil
	})
}

func (c *CLI) upgradeProxyOn(hosts []string, opts, invokeOptions Options) error {
	list := strings.Join(hosts, ",")
	c.say("Upgrading proxy on "+list+"...", "magenta")
	if err := c.runHook(opts, "pre-proxy-reboot", false, tags.Tag{Name: "hosts", Value: list}); err != nil {
		return err
	}
	err := c.on(opts, hosts, func(s *sshexec.Session) error {
		proxy := c.kamal.Proxy(s.Host)
		login, err := c.loginCmds()
		if err != nil {
			return err
		}
		steps := append(append([]sshexec.Command{debugCmd(c.kamal.Auditor().Record("Rebooted proxy"))}, login...),
			cmd(proxy.CleanupTraefik()), lenientCmd(proxy.Stop("")), cmd(proxy.RemoveContainer()), cmd(proxy.RemoveImage()))
		return executeAll(s, steps...)
	})
	if err != nil {
		return err
	}
	if err := c.kamal.WithSpecificHosts(hosts, func() error { return c.rebootAfterUpgrade(invokeOptions) }); err != nil {
		return err
	}
	if err := c.runHook(opts, "post-proxy-reboot", false, tags.Tag{Name: "hosts", Value: list}); err != nil {
		return err
	}
	c.say("Upgraded proxy on "+list, "magenta")
	return nil
}

func (c *CLI) rebootAfterUpgrade(invokeOptions Options) error {
	for _, step := range []struct{ name, group string }{{"proxy:boot", "proxy"}, {"app:boot", "app"}, {"prune:all", "prune"}} {
		if err := c.invoke(step.name, nil, invokeOptions); err != nil {
			return err
		}
		c.resetInvocation(step.group)
	}
	return nil
}

func (c *CLI) proxyOnHosts(opts Options, audit string, lenient bool, line func(host string) shell.Line) error {
	return c.modify(true, func() error {
		return c.on(opts, c.kamal.ProxyHosts(), func(s *sshexec.Session) error {
			if audit != "" {
				if err := s.Execute(debugCmd(c.kamal.Auditor().Record(audit))); err != nil {
					return err
				}
			}
			if lenient {
				return s.Execute(lenientCmd(line(s.Host)))
			}
			return s.Execute(cmd(line(s.Host)))
		})
	})
}

func (c *CLI) proxyStart(opts Options) error {
	return c.proxyOnHosts(opts, "Started proxy", false, func(h string) shell.Line { return c.kamal.Proxy(h).Start() })
}

func (c *CLI) proxyStop(opts Options) error {
	return c.proxyOnHosts(opts, "Stopped proxy", true, func(h string) shell.Line { return c.kamal.Proxy(h).Stop("") })
}

func (c *CLI) proxyRestart(opts Options) error {
	return c.modify(true, func() error {
		if err := c.proxyStop(opts); err != nil {
			return err
		}
		return c.proxyStart(opts)
	})
}

func (c *CLI) proxyDetails(opts Options) error {
	return c.on(opts, c.kamal.ProxyHosts(), func(s *sshexec.Session) error {
		output, err := s.CaptureWithInfo(cmd(c.kamal.Proxy(s.Host).Info()))
		c.putsByHost(s.Host, output, "Proxy", opts.Bool("quiet"), false)
		return err
	})
}

func (c *CLI) proxyLogs(opts Options) error {
	logOpts := logOptionsFrom(opts)
	if opts.Bool("follow") {
		host := c.kamal.PrimaryHost()
		follow := c.kamal.Proxy(host).FollowLogs(host, logOpts)
		c.syncVerbosity()
		c.out.Log(sshexec.Info, "Following logs on "+host+"...")
		c.out.Log(sshexec.Info, follow)
		return c.rt.Exec(follow)
	}
	logOpts.Lines = defaultLines(opts, 100)
	return c.on(opts, c.kamal.ProxyHosts(), func(s *sshexec.Session) error {
		output, err := s.Capture(cmd(c.kamal.Proxy(s.Host).Logs(logOpts)))
		c.putsByHost(s.Host, output, "Proxy", false, false)
		return err
	})
}

func (c *CLI) proxyRemove(opts Options) error {
	return c.modify(true, func() error {
		allowed, err := c.removalAllowed(opts)
		if err != nil || !allowed {
			return err
		}
		return runSteps([]func() error{
			func() error { return c.proxyStop(opts) }, func() error { return c.proxyRemoveContainer(opts) },
			func() error { return c.proxyRemoveImage(opts) }, func() error { return c.proxyRemoveProxyDirectory(opts) },
		})
	})
}

const otherAppsInstalled = "The are other applications installed on"

func (c *CLI) removalAllowed(opts Options) (bool, error) {
	err := c.on(opts, c.kamal.ProxyHosts(), func(s *sshexec.Session) error {
		count, err := s.CaptureWithInfo(cmd(c.kamal.Server().AppDirectoryCount()))
		if err != nil {
			return err
		}
		if n, _ := strconv.Atoi(strings.TrimSpace(count)); n > 0 {
			return errors.New(otherAppsInstalled + " " + s.Host)
		}
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), otherAppsInstalled) {
		return err == nil, err
	}
	if opts.Bool("force") {
		c.say("Forcing, so removing the proxy, even though other apps are installed", "magenta")
		return true, nil
	}
	c.say("Not removing the proxy, as other apps are installed, ignore this check with kamal proxy remove --force", "magenta")
	return false, nil
}

func (c *CLI) proxyRemoveContainer(opts Options) error {
	return c.proxyOnHosts(opts, "Removed proxy container", false, func(h string) shell.Line { return c.kamal.Proxy(h).RemoveContainer() })
}

func (c *CLI) proxyRemoveImage(opts Options) error {
	return c.proxyOnHosts(opts, "Removed proxy image", false, func(h string) shell.Line { return c.kamal.Proxy(h).RemoveImage() })
}

func (c *CLI) proxyRemoveProxyDirectory(opts Options) error {
	return c.proxyOnHosts(opts, "", true, func(h string) shell.Line { return c.kamal.Proxy(h).RemoveProxyDirectory() })
}

func logOptionsFrom(opts Options) commands.LogOptions {
	return commands.LogOptions{Timestamps: !opts.Bool("skip_timestamps"), Since: opts.String("since"), Grep: opts.String("grep")}
}

func nonEmptyStrings(parts ...string) []string {
	var out []string
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func wordsText(line shell.Line) []string {
	out := make([]string, len(line))
	for i, word := range line {
		out[i] = word.String()
	}
	return out
}
