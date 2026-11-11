package cli

// Ports run_hook and the invocation tracking of lib/kamal/cli/base.rb at Kamal 2.12.0.

import (
	"errors"
	"strconv"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/sshexec"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/tags"
)

// runHook runs a local hook with the KAMAL_* variables, unless --skip-hooks is set or
// the hook file is missing. extra adds details, such as {"runtime", "12"}.
func (c *CLI) runHook(opts Options, hook string, withSecrets bool, extra ...tags.Tag) error {
	if opts.Bool("skip_hooks") || !c.kamal.Hook().HookExists(hook) {
		return nil
	}
	env, err := c.kamal.Hook().Env(withSecrets, append(c.hookDetails(), extra...)...)
	if err != nil {
		return err
	}
	return c.kamal.WithVerbosity(c.hookVerbosity(hook), func() error {
		err := c.runLocally(processEnv(env), func(s *sshexec.Session) error {
			return s.Execute(cmd(c.kamal.Hook().Run(hook)))
		})
		var failed *sshexec.FailedError
		if errors.As(err, &failed) {
			return hookError("Hook `" + hook + "` failed:\n" + err.Error())
		}
		return err
	})
}

// hookDetails returns hosts, roles when selected, lock, command, and subcommand.
func (c *CLI) hookDetails() []tags.Tag {
	details := []tags.Tag{{Name: "hosts", Value: strings.Join(c.kamal.Hosts(), ",")}}
	if roles := c.kamal.SpecificRoles(); roles != nil {
		var names []string
		for _, role := range roles {
			names = append(names, role.Name())
		}
		details = append(details, tags.Tag{Name: "roles", Value: strings.Join(names, ",")})
	}
	details = append(details, tags.Tag{Name: "lock", Value: strconv.FormatBool(c.kamal.HoldingLock)})
	if command := c.command(); command != "" {
		details = append(details, tags.Tag{Name: "command", Value: command})
	}
	if subcommand := c.subcommand(); subcommand != "" {
		details = append(details, tags.Tag{Name: "subcommand", Value: subcommand})
	}
	return details
}

// hookVerbosity follows Kamal: -q hides hooks and -v shows them, and otherwise the
// hooks_output setting decides.
func (c *CLI) hookVerbosity(hook string) sshexec.Level {
	output := c.config().HooksOutputFor(hook)
	if c.kamal.Verbosity == sshexec.Info && output != "" {
		return verbosityLevels[output]
	}
	return c.kamal.Verbosity
}

func processEnv(env []tags.Tag) []sshexec.EnvVar {
	out := make([]sshexec.EnvVar, len(env))
	for i, tag := range env {
		out[i] = sshexec.EnvVar{Key: tag.Name, Value: tag.Value}
	}
	return out
}

// command returns the first command of the run, such as deploy, or app for
// `kamal app boot`.
func (c *CLI) command() string {
	if len(c.first) == 0 {
		return ""
	}
	return c.first[0]
}

// subcommand returns the subcommand of the run, such as boot for `kamal app boot`.
func (c *CLI) subcommand() string {
	if len(c.first) < 2 {
		return ""
	}
	return c.first[1]
}
