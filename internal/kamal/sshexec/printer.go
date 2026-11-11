package sshexec

// Ports SSHKit::Backend::Printer from SSHKit 1.25.1 and the upload override in
// Kamal's test/test_helper.rb.

import (
	"fmt"
	"strconv"
)

// Printer logs each command and runs nothing, as the backend of Kamal's CLI tests.
// Captures return Stdout for a command when CapturedOutput holds one.
type Printer struct {
	Print          func(string)
	CapturedOutput map[string]string
}

// Run logs the start of cmd and returns the scripted output, if any.
func (p *Printer) Run(host string, cmd Command, id string, out *Formatter) (Result, error) {
	out.commandStart(cmd, host, id)
	return Result{Stdout: p.CapturedOutput[cmd.ToCommand()]}, nil
}

// Upload prints what it would send, as Kamal's tests do.
func (p *Printer) Upload(host string, content Upload, remote string, out *Formatter) error {
	local := content.LocalPath
	if content.InMemory {
		local = strconv.Quote(string(content.Content))
	}
	p.Print(fmt.Sprintf("Uploading %s to %s on %s\n", local, remote, host))
	return nil
}

// LogsExit is false: the printer logs only the command start.
func (p *Printer) LogsExit() bool { return false }

// AlwaysSucceeds is true: every test command passes.
func (p *Printer) AlwaysSucceeds() bool { return true }
