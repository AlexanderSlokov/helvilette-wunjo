package cli

// Ports the exec and logs commands of lib/kamal/cli/app.rb at Kamal 2.12.0.

import (
	"strconv"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/commands"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/sshexec"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/tags"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
)

func (c *CLI) appExec(args []string, opts Options) error {
	if err := checkExecOptions(opts, args); err != nil {
		return err
	}
	return c.withRawOutput(opts.Bool("raw"), func() error {
		if err := c.preConnectIfRequired(opts); err != nil {
			return err
		}
		command := utils.JoinCommands(args)
		switch {
		case opts.Bool("interactive") && opts.Bool("reuse"):
			return c.execInteractiveReuse(opts, command)
		case opts.Bool("interactive"):
			return c.execInteractiveNew(opts, command)
		case opts.Bool("reuse"):
			return c.execReuse(opts, command)
		default:
			return c.execNew(opts, command)
		}
	})
}

func (c *CLI) reuseVersion(opts Options) (string, error) {
	if version := opts.String("version"); version != "" {
		return version, nil
	}
	c.say("Get current version of running container...", "magenta")
	return c.currentRunningVersion(opts, c.kamal.PrimaryHost())
}

func (c *CLI) latestVersionAnnounced(opts Options) string {
	if !opts.Has("version") {
		c.say("Get most recent version available as an image...", "magenta")
	}
	return c.versionOrLatest(opts)
}

func (c *CLI) execInteractiveReuse(opts Options, command string) error {
	version, err := c.reuseVersion(opts)
	if err != nil {
		return err
	}
	return c.usingVersion(version, func(version string) error {
		c.say("Launching interactive command with version "+version+" via SSH from existing container on "+c.kamal.PrimaryHost()+"...", "magenta")
		app := c.kamal.App(c.kamal.PrimaryRole(), c.kamal.PrimaryHost())
		return c.rt.Exec(app.ExecuteInExistingContainerOverSSH([]string{command}, execEnv(opts)))
	})
}

func (c *CLI) execInteractiveNew(opts Options, command string) error {
	return c.usingVersion(c.latestVersionAnnounced(opts), func(version string) error {
		c.say("Launching interactive command with version "+version+" via SSH from new container on "+c.kamal.PrimaryHost()+"...", "magenta")
		if err := c.registryLoginOn(opts, c.primaryHosts()); err != nil {
			return err
		}
		app := c.kamal.App(c.kamal.PrimaryRole(), c.kamal.PrimaryHost())
		return c.rt.Exec(app.ExecuteInNewContainerOverSSH([]string{command}, execEnv(opts)))
	})
}

func (c *CLI) execReuse(opts Options, command string) error {
	version, err := c.reuseVersion(opts)
	if err != nil {
		return err
	}
	return c.usingVersion(version, func(version string) error {
		c.say("Launching command with version "+version+" from existing container...", "magenta")
		return c.onRoles(opts, c.kamal.Roles(), c.kamal.AppHosts(), true, func(s *sshexec.Session, role *configuration.Role) error {
			audit := c.kamal.Auditor(tags.Tag{Name: "role", Value: role.Name()}).Record("Executed cmd '" + command + "' on app version " + version)
			if err := s.Execute(debugCmd(audit)); err != nil {
				return err
			}
			return c.captureExecOutput(s, opts, c.kamal.App(role, s.Host).ExecuteInExistingContainer([]string{command}, false, execEnv(opts)))
		})
	})
}

func (c *CLI) execNew(opts Options, command string) error {
	return c.usingVersion(c.latestVersionAnnounced(opts), func(version string) error {
		c.say("Launching command with version "+version+" from new container...", "magenta")
		if err := c.registryLoginOn(opts, c.kamal.AppHosts()); err != nil {
			return err
		}
		return c.onRoles(opts, c.kamal.Roles(), c.kamal.AppHosts(), true, func(s *sshexec.Session, role *configuration.Role) error {
			if err := s.Execute(debugCmd(c.kamal.Auditor().Record("Executed cmd '" + command + "' on app version " + version))); err != nil {
				return err
			}
			line := c.kamal.App(role, s.Host).ExecuteInNewContainer([]string{command}, false, opts.Bool("detach"), execEnv(opts))
			return c.captureExecOutput(s, opts, line)
		})
	})
}

// captureExecOutput prints the output of an exec under the host, unstripped for raw.
func (c *CLI) captureExecOutput(s *sshexec.Session, opts Options, line commands.Line) error {
	raw := opts.Bool("raw")
	output, err := s.CaptureWithInfoRaw(cmd(line), raw)
	c.putsByHost(s.Host, output, "App", opts.Bool("quiet"), raw)
	return err
}

func (c *CLI) registryLoginOn(opts Options, hosts []string) error {
	return c.on(opts, hosts, func(s *sshexec.Session) error {
		line, err := c.kamal.Registry().Login(nil)
		if err != nil || line == nil {
			return err
		}
		return s.Execute(cmd(line))
	})
}

func (c *CLI) appLogs(opts Options) error {
	logOpts := commands.LogOptions{ContainerID: opts.String("container_id"), Timestamps: !opts.Bool("skip_timestamps"),
		Since: opts.String("since"), Grep: opts.String("grep"), GrepOptions: opts.String("grep_options")}
	if opts.Bool("follow") {
		logOpts.Lines = defaultLines(opts, 10)
		return c.followAppLogs(logOpts)
	}
	logOpts.Lines = defaultLines(opts, 100)
	return c.onRoles(opts, c.kamal.Roles(), c.kamal.AppHosts(), true, func(s *sshexec.Session, role *configuration.Role) error {
		output, err := s.CaptureWithInfo(cmd(c.kamal.App(role, s.Host).Logs(logOpts)))
		if _, failed := err.(*sshexec.FailedError); failed {
			output, err = "Nothing found", nil
		}
		c.putsByHost(s.Host, output, "App", opts.Bool("quiet"), false)
		return err
	})
}

// defaultLines returns --lines, or fallback when neither --since nor --grep is set.
func defaultLines(opts Options, fallback int) string {
	if lines, ok := opts.Int("lines"); ok {
		return strconv.FormatInt(lines, 10)
	}
	if opts.String("since") != "" || opts.String("grep") != "" {
		return ""
	}
	return strconv.Itoa(fallback)
}

func (c *CLI) followAppLogs(logOpts commands.LogOptions) error {
	host := c.kamal.PrimaryHost()
	c.syncVerbosity()
	c.out.Log(sshexec.Info, "Following logs on "+host+"...")
	if c.kamal.SpecificRoles() == nil {
		if err := c.kamal.SetSpecificRoles([]string{c.kamal.PrimaryRole().Name()}); err != nil {
			return err
		}
	}
	role := c.kamal.RolesOn(host)[0]
	follow := c.kamal.App(role, sshexec.LocalHost).FollowLogs(host, logOpts)
	c.out.Log(sshexec.Info, follow)
	return c.rt.Exec(follow)
}
