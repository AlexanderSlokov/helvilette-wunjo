// Package sshexec runs Kamal's commands on hosts over SSH and on the local machine,
// as SSHKit does for Kamal. It renders each command the way SSHKit 1.25 does, logs
// it in SSHKit's pretty format, and runs it with the operator's OpenSSH client, so
// ~/.ssh/config, ProxyJump, and the SSH agent behave as they do for `ssh`.
//
// Ports the parts of SSHKit 1.25.1 that Kamal uses and lib/kamal/sshkit_with_ext.rb
// at Kamal 2.12.0.
package sshexec

import (
	"fmt"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
)

// Level is a log level, as Ruby's Logger levels: Debug 0 to Fatal 4.
type Level int

const (
	// Debug logs command output and the full command line.
	Debug Level = iota
	// Info logs each command and its exit.
	Info
	// Warn logs warnings.
	Warn
	// Error logs errors.
	Error
	// Fatal logs fatal errors.
	Fatal
)

// EnvVar is one variable that a command exports before it runs.
type EnvVar struct {
	Key   string
	Value string
}

// Command is one command line with its options, as SSHKit::Command.
type Command struct {
	Line               shell.Line
	Env                []EnvVar
	Verbosity          Level
	RaiseOnNonZeroExit bool
}

// NewCommand returns a command that logs at Info and fails on a non-zero exit, as
// SSHKit's execute does.
func NewCommand(line shell.Line) Command {
	return Command{Line: line, Verbosity: Info, RaiseOnNonZeroExit: true}
}

// name returns the command name: the first word with its lines joined by "; ", as
// SSHKit's sanitize_command does.
func (c Command) name() string {
	if len(c.Line) == 0 {
		return ""
	}
	return sanitize(c.Line[0].String())
}

func sanitize(text string) string {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	return strings.Join(lines, "; ")
}

// shouldMap reports whether the name is one word. A name with whitespace is a
// complete shell command, which SSHKit runs as written and without its arguments.
func (c Command) shouldMap() bool { return !strings.ContainsAny(c.name(), " \t\n\r\f\v") }

// mappedName returns the name as SSHKit's command map renders it: docker as is,
// because Kamal maps it to itself, and /usr/bin/env <name> for every other command.
func (c Command) mappedName() string {
	if c.name() == "docker" {
		return "docker"
	}
	return "/usr/bin/env " + c.name()
}

// String returns the command as SSHKit's Command#to_s, with sensitive words shown.
func (c Command) String() string { return c.render(shell.Word.String) }

// Redacted returns the command as SSHKit logs it, with sensitive words redacted.
func (c Command) Redacted() string { return c.render(shell.Word.Redacted) }

func (c Command) render(form func(shell.Word) string) string {
	if !c.shouldMap() {
		return c.name()
	}
	parts := []string{c.mappedName()}
	for _, word := range c.Line[1:] {
		parts = append(parts, form(word))
	}
	return strings.Join(parts, " ")
}

// ToCommand returns the shell text that runs: the command, wrapped in
// ( export K="v" ; ... ) when it has an env, as SSHKit's Command#to_command.
func (c Command) ToCommand() string {
	if !c.shouldMap() || len(c.Env) == 0 {
		return c.String()
	}
	return "( export " + c.environmentString() + " ; " + c.String() + " )"
}

func (c Command) environmentString() string {
	parts := make([]string, len(c.Env))
	for i, v := range c.Env {
		parts[i] = v.Key + `="` + strings.ReplaceAll(v.Value, `"`, `\"`) + `"`
	}
	return strings.Join(parts, " ")
}

// FailedError reports a command that exited with a non-zero status, with SSHKit's
// message.
type FailedError struct {
	Name       string
	ExitStatus int
	Stdout     string
	Stderr     string
}

func (e *FailedError) Error() string {
	return fmt.Sprintf("%s exit status: %d\n%s stdout: %s\n%s stderr: %s\n",
		e.Name, e.ExitStatus, e.Name, orNothing(e.Stdout), e.Name, orNothing(e.Stderr))
}

func orNothing(text string) string {
	if strings.TrimSpace(text) == "" {
		return "Nothing written"
	}
	return strings.TrimSpace(text)
}
