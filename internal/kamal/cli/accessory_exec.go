package cli

// Ports the exec and logs commands of lib/kamal/cli/accessory.rb at Kamal 2.12.0.

import (
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/commands"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/sshexec"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
)

func (c *CLI) accessoryExec(args []string, opts Options) error {
	name := args[0]
	if opts.Bool("raw") && opts.Bool("interactive") {
		return argumentError("Raw is not compatible with interactive")
	}
	return c.withRawOutput(opts.Bool("raw"), func() error {
		if err := c.preConnectIfRequired(opts); err != nil {
			return err
		}
		command := utils.JoinCommands(args[1:])
		return c.withAccessory(name, func(accessory *commands.Accessory, hosts []string) error {
			return c.accessoryExecMode(name, accessory, hosts, command, opts)
		})
	})
}

func (c *CLI) accessoryExecMode(name string, accessory *commands.Accessory, hosts []string, command string, opts Options) error {
	switch {
	case opts.Bool("interactive") && opts.Bool("reuse"):
		c.say("Launching interactive command via SSH from existing container...", "magenta")
		return c.rt.Exec(accessory.ExecuteInExistingContainerOverSSH([]string{command}))
	case opts.Bool("interactive"):
		return c.accessoryInteractiveNew(name, accessory, command, opts)
	case opts.Bool("reuse"):
		c.say("Launching command from existing container...", "magenta")
		return c.accessoryExecOn(name, hosts, command, opts, false, func() (commands.Line, error) {
			return accessory.ExecuteInExistingContainer([]string{command}, false), nil
		})
	default:
		c.say("Launching command from new container...", "magenta")
		return c.accessoryExecOn(name, hosts, command, opts, true, func() (commands.Line, error) {
			return accessory.ExecuteInNewContainer([]string{command}, false)
		})
	}
}

func (c *CLI) accessoryInteractiveNew(name string, accessory *commands.Accessory, command string, opts Options) error {
	c.say("Launching interactive command via SSH from new container...", "magenta")
	if err := c.registryLoginOn(opts, c.config().Accessory(name).Hosts()[:1]); err != nil {
		return err
	}
	ssh, err := accessory.ExecuteInNewContainerOverSSH([]string{command})
	if err != nil {
		return err
	}
	return c.rt.Exec(ssh)
}

func (c *CLI) accessoryExecOn(name string, hosts []string, command string, opts Options, login bool, line func() (commands.Line, error)) error {
	return c.on(opts, hosts, func(s *sshexec.Session) error {
		if login {
			if err := c.executeLogin(s); err != nil {
				return err
			}
		}
		if err := s.Execute(debugCmd(c.kamal.Auditor().Record("Executed cmd '" + command + "' on " + name + " accessory"))); err != nil {
			return err
		}
		exec, err := line()
		if err != nil {
			return err
		}
		return c.captureExecOutput(s, opts, exec)
	})
}

func (c *CLI) executeLogin(s *sshexec.Session) error {
	login, err := c.kamal.Registry().Login(nil)
	if err != nil || login == nil {
		return err
	}
	return s.Execute(cmd(login))
}

func (c *CLI) accessoryLogs(name string, opts Options) error {
	return c.withAccessory(name, func(accessory *commands.Accessory, hosts []string) error {
		logOpts := commands.LogOptions{Timestamps: !opts.Bool("skip_timestamps"), Since: opts.String("since"), Grep: opts.String("grep"), GrepOptions: opts.String("grep_options")}
		if opts.Bool("follow") {
			return c.followAccessoryLogs(accessory, hosts, logOpts, opts)
		}
		logOpts.Lines = defaultLines(opts, 100)
		return c.on(opts, hosts, func(s *sshexec.Session) error {
			output, err := s.CaptureWithInfo(cmd(accessory.Logs(logOpts)))
			c.puts(output)
			return err
		})
	})
}

func (c *CLI) followAccessoryLogs(accessory *commands.Accessory, hosts []string, logOpts commands.LogOptions, opts Options) error {
	if len(hosts) == 0 {
		return nil
	}
	logOpts.Lines = defaultLines(opts, 10)
	c.syncVerbosity()
	c.out.Log(sshexec.Info, "Following logs on "+hosts[0]+"...")
	follow := accessory.FollowLogs(hosts[0], logOpts)
	c.out.Log(sshexec.Info, follow)
	return c.rt.Exec(follow)
}
