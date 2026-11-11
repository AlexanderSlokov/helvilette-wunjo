// Package localshell runs commands on the operator's machine, as Ruby's backticks and
// Kernel#system do in Kamal. Callers receive a Shell, so tests pass a fake.
//
// Ports the backtick and system calls in lib/kamal/git.rb, lib/kamal/utils.rb, and
// lib/kamal/secrets/dotenv/inline_command_substitution.rb at Kamal 2.12.0.
package localshell

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
)

// Result is the outcome of one command: its standard output and whether it exited
// with status 0.
type Result struct {
	Stdout  string
	Success bool
}

// Shell runs one command line through /bin/sh.
type Shell interface {
	Run(command string) (Result, error)
}

// System runs commands with /bin/sh -c in Dir. Standard error goes to Stderr, as it
// does for Ruby's backticks.
//
//	result, err := localshell.System{Stderr: os.Stderr}.Run("git rev-parse HEAD")
type System struct {
	Dir    string
	Stderr io.Writer
}

// Run executes command and returns its output. A non-zero exit status is a result
// with Success false. An error means the shell itself could not start.
func (s System) Run(command string) (Result, error) {
	cmd := exec.Command("/bin/sh", "-c", command)
	cmd.Dir = s.Dir
	cmd.Stderr = s.stderr()
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return Result{Stdout: stdout.String(), Success: false}, nil
	}
	return Result{Stdout: stdout.String(), Success: err == nil}, err
}

func (s System) stderr() io.Writer {
	if s.Stderr == nil {
		return os.Stderr
	}
	return s.Stderr
}
