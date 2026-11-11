package cli

// Ports lib/kamal/cli/base.rb at Kamal 2.12.0.

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/sshexec"
)

// verbosityLevels maps --verbose and --quiet, and hooks_output, to log levels.
var verbosityLevels = map[string]sshexec.Level{"verbose": sshexec.Debug, "quiet": sshexec.Error}

// config returns the loaded configuration. The commander loads it before any command
// body runs, so a failure there has already been reported.
func (c *CLI) config() *configuration.Configuration { return c.kamal.MustConfig() }

// say prints a message, colored when stdout is a terminal, as Thor's say does: a
// newline follows unless the message ends with a space or a tab.
func (c *CLI) say(message, color string) {
	text := message
	if color != "" && c.rt.StdoutTTY {
		text = thorColor(color) + message + "\033[0m"
	}
	if !strings.HasSuffix(message, " ") && !strings.HasSuffix(message, "\t") {
		text += "\n"
	}
	fmt.Fprint(c.rt.Stdout, text)
}

func thorColor(color string) string {
	codes := map[string]string{"red": "31", "green": "32", "yellow": "33", "magenta": "35", "blue": "34"}
	return "\033[" + codes[color] + "m"
}

// puts prints a line, as Ruby's puts does.
func (c *CLI) puts(text string) {
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	fmt.Fprint(c.rt.Stdout, text)
}

// printRuntime runs fn and prints how long it took, even when it fails.
func (c *CLI) printRuntime(fn func() error) (time.Duration, error) {
	started := c.rt.Now()
	err := fn()
	runtime := c.rt.Now().Sub(started)
	c.puts(fmt.Sprintf("  Finished all in %.1f seconds", runtime.Seconds()))
	return runtime, err
}

// modify runs fn as a change to the hosts, holding the deploy lock when lock is set.
func (c *CLI) modify(lock bool, fn func() error) error {
	if lock {
		return c.withLock(fn)
	}
	return fn()
}

// confirming runs fn when --confirmed is set or the user answers y.
func (c *CLI) confirming(opts Options, question string, fn func() error) error {
	if opts.Bool("confirmed") {
		return fn()
	}
	if c.ask(question, []string{"y", "N"}, "N") == "y" {
		return fn()
	}
	c.say("Aborted", "red")
	return nil
}

// ask prints Thor's limited question, "<question> [y, N] (N) ", until the answer is
// one of the choices. An empty answer takes the default.
func (c *CLI) ask(question string, choices []string, fallback string) string {
	reader := bufio.NewReader(c.rt.Stdin)
	for {
		fmt.Fprintf(c.rt.Stdout, "%s [%s] (%s) ", question, strings.Join(choices, ", "), fallback)
		line, err := reader.ReadString('\n')
		answer := strings.TrimSpace(line)
		if answer == "" {
			answer = fallback
		}
		if containsString(choices, answer) || err != nil {
			return answer
		}
		c.puts("Your response must be one of: [" + strings.Join(choices, ", ") + "]. Please try again.")
	}
}

// on runs fn on hosts in parallel, after the pre-connect hook.
func (c *CLI) on(opts Options, hosts []string, fn func(*sshexec.Session) error) error {
	if err := c.preConnectIfRequired(opts); err != nil {
		return err
	}
	return c.runner(c.remoteBackend()).On(hosts, fn)
}

// onRoles runs fn per host and role, after the pre-connect hook.
func (c *CLI) onRoles(opts Options, roles []*configuration.Role, hosts []string, parallel bool, fn func(*sshexec.Session, *configuration.Role) error) error {
	if err := c.preConnectIfRequired(opts); err != nil {
		return err
	}
	byName := map[string]*configuration.Role{}
	var list []sshexec.RoleHosts
	for _, role := range roles {
		byName[role.Name()] = role
		list = append(list, sshexec.RoleHosts{Role: role.Name(), Hosts: role.Hosts()})
	}
	return c.runner(c.remoteBackend()).OnRoles(list, hosts, parallel, func(s *sshexec.Session, name string) error {
		return fn(s, byName[name])
	})
}

// inGroups runs fn on hosts in groups of limit, waiting between groups.
func (c *CLI) inGroups(opts Options, hosts []string, limit int, fn func(*sshexec.Session) error) error {
	if err := c.preConnectIfRequired(opts); err != nil {
		return err
	}
	return c.runner(c.remoteBackend()).InGroups(hosts, limit, 0, fn)
}

// runLocally runs fn on this machine, with the variables of withEnv.
func (c *CLI) runLocally(env []sshexec.EnvVar, fn func(*sshexec.Session) error) error {
	c.syncVerbosity()
	s := sshexec.NewSession(sshexec.LocalHost, c.rt.Local, c.out, c.newID)
	s.ProcessEnv = env
	return fn(s)
}

func (c *CLI) runner(backend sshexec.Backend) *sshexec.Runner {
	c.syncVerbosity()
	return &sshexec.Runner{Backend: backend, Out: c.out, NewID: c.newID, Sleep: c.rt.Sleep}
}

func (c *CLI) syncVerbosity() { c.out.Verbosity = c.kamal.Verbosity }

func (c *CLI) newID() string { return c.rt.RandomHex(4) }

func (c *CLI) remoteBackend() sshexec.Backend {
	if c.remote == nil {
		c.remote = c.rt.Remote(c.config())
	}
	return c.remote
}

// preConnectIfRequired runs the pre-connect hook once, before the first connection.
func (c *CLI) preConnectIfRequired(opts Options) error {
	if c.kamal.Connected {
		return nil
	}
	c.kamal.Connected = true
	if opts.Bool("skip_hooks") {
		return nil
	}
	return c.runHook(opts, "pre-connect", true)
}

// ensureRunDirectory creates .kamal on every selected host.
func (c *CLI) ensureRunDirectory(opts Options) error {
	return c.on(opts, c.kamal.Hosts(), func(s *sshexec.Session) error {
		return s.Execute(cmd(c.kamal.Server().EnsureRunDirectory()))
	})
}

// ensureDockerInstalled checks the local docker and buildx.
func (c *CLI) ensureDockerInstalled() error {
	return c.runLocally(nil, func(s *sshexec.Session) error {
		err := s.Execute(cmd(c.kamal.Builder().EnsureDockerInstalled()))
		if err == nil {
			return nil
		}
		if strings.Contains(err.Error(), "command not found") {
			return dependencyError("Docker is not installed locally")
		}
		return dependencyError("Docker buildx plugin is not installed locally")
	})
}

// withRawOutput silences command echoing while fn writes raw output for piping.
func (c *CLI) withRawOutput(raw bool, fn func() error) error {
	if !raw {
		return fn()
	}
	return c.kamal.WithVerbosity(sshexec.Error, fn)
}

// words returns plain words, for example words("bundle", "add", "kamal").
func words(texts ...string) shell.Line { return shell.Words(texts...) }

// cmd builds a command that logs at Info and fails on a non-zero exit.
func cmd(line shell.Line) sshexec.Command { return sshexec.NewCommand(line) }

// debugCmd builds a command that logs at Debug, as verbosity: :debug.
func debugCmd(line shell.Line) sshexec.Command {
	c := sshexec.NewCommand(line)
	c.Verbosity = sshexec.Debug
	return c
}

// lenientCmd builds a command that tolerates a non-zero exit, as
// raise_on_non_zero_exit: false.
func lenientCmd(line shell.Line) sshexec.Command {
	c := sshexec.NewCommand(line)
	c.RaiseOnNonZeroExit = false
	return c
}

func containsString(list []string, item string) bool {
	for _, s := range list {
		if s == item {
			return true
		}
	}
	return false
}
