package sshexec

// Ports SSHKit::Backend::Abstract from SSHKit 1.25.1 and the backend extensions of
// lib/kamal/sshkit_with_ext.rb at Kamal 2.12.0.

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Result is the outcome of one command.
type Result struct {
	Stdout     string
	Stderr     string
	ExitStatus int
	Runtime    time.Duration
	// Completed reports that the command ran to an exit status. The printer of the
	// tests never runs commands, so their results are not completed.
	Completed bool
}

// Backend runs commands for a Session: over SSH, locally, or only printing them.
type Backend interface {
	// Run logs the start of cmd, runs it on host, and logs its output.
	Run(host string, cmd Command, id string, out *Formatter) (Result, error)
	// Upload writes content to remote on host, with mode such as "755" when set.
	Upload(host string, content Upload, remote string, out *Formatter) error
	// LogsExit reports whether the backend logs the exit line, which the printer
	// backend of the tests does not.
	LogsExit() bool
	// AlwaysSucceeds reports whether test commands always pass, as the printer does.
	AlwaysSucceeds() bool
}

// Upload is what Session.Upload sends: a local file, content held in memory, or a
// directory tree held in memory, which lands as <remote>/<base of LocalPath>, as a
// recursive SCP upload does.
type Upload struct {
	LocalPath string
	Content   []byte
	InMemory  bool
	Tree      map[string][]byte
	Mode      string
}

// Session runs commands on one host, as an SSHKit backend inside an `on` block.
type Session struct {
	Host    string
	backend Backend
	out     *Formatter
	newID   func() string
	env     []EnvVar
	// ProcessEnv adds variables to the environment of local commands.
	ProcessEnv []EnvVar
}

// NewSession returns a session on host. newID returns the 8-character ID that
// prefixes each logged command.
func NewSession(host string, backend Backend, out *Formatter, newID func() string) *Session {
	return &Session{Host: host, backend: backend, out: out, newID: newID}
}

// Execute runs cmd and fails on a non-zero exit, as execute does.
func (s *Session) Execute(cmd Command) error {
	_, err := s.run(cmd)
	return err
}

// ExecuteOK runs cmd without failing and reports whether it completed with status
// 0, as execute(..., raise_on_non_zero_exit: false) returns success?.
func (s *Session) ExecuteOK(cmd Command) (bool, error) {
	cmd.RaiseOnNonZeroExit = false
	result, err := s.run(cmd)
	return err == nil && result.Completed && result.ExitStatus == 0, err
}

// Capture runs cmd at Debug and returns its stripped output, as capture does.
func (s *Session) Capture(cmd Command) (string, error) {
	cmd.Captures = true
	cmd.Verbosity = Debug
	result, err := s.run(cmd)
	return strings.TrimSpace(result.Stdout), err
}

// CaptureWithInfo runs cmd at Info and returns its stripped output.
func (s *Session) CaptureWithInfo(cmd Command) (string, error) {
	cmd.Captures = true
	cmd.Verbosity = Info
	result, err := s.run(cmd)
	return strings.TrimSpace(result.Stdout), err
}

// CaptureWithInfoRaw runs cmd at Info and returns its output, stripped unless raw.
func (s *Session) CaptureWithInfoRaw(cmd Command, raw bool) (string, error) {
	cmd.Captures = true
	cmd.Verbosity = Info
	result, err := s.run(cmd)
	if raw {
		return result.Stdout, err
	}
	return strings.TrimSpace(result.Stdout), err
}

// CaptureWithPrettyJSON returns the output of cmd as indented JSON, as
// capture_with_pretty_json does.
func (s *Session) CaptureWithPrettyJSON(cmd Command) (string, error) {
	output, err := s.Capture(cmd)
	if err != nil {
		return "", err
	}
	var parsed json.RawMessage
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return "", fmt.Errorf("output of %q is not JSON: %w", cmd.Redacted(), err)
	}
	pretty, err := json.MarshalIndent(parsed, "", "  ")
	return string(pretty), err
}

// Test runs cmd at Debug and reports whether it exited with status 0.
func (s *Session) Test(cmd Command) bool {
	cmd.Verbosity, cmd.RaiseOnNonZeroExit = Debug, false
	result, err := s.run(cmd)
	return s.backend.AlwaysSucceeds() || (err == nil && result.ExitStatus == 0)
}

// Upload sends a file or in-memory content to remote.
func (s *Session) Upload(content Upload, remote string) error {
	return s.backend.Upload(s.Host, content, remote, s.out)
}

// With runs fn with env added to every command, as SSHKit's with does.
func (s *Session) With(env []EnvVar, fn func() error) error {
	previous := s.env
	s.env = append(append([]EnvVar{}, previous...), env...)
	defer func() { s.env = previous }()
	return fn()
}

// Log writes a message at level, as info, warn, and debug do inside an on block.
func (s *Session) Log(level Level, message string) { s.out.Log(level, message) }

// run runs cmd, raising SSHKit's Failed error before the exit line is logged.
func (s *Session) run(cmd Command) (Result, error) {
	cmd.Env = append(append([]EnvVar{}, s.env...), cmd.Env...)
	cmd.ProcessEnv = append(append([]EnvVar{}, s.ProcessEnv...), cmd.ProcessEnv...)
	id := s.newID()
	result, err := s.backend.Run(s.Host, cmd, id, s.out)
	if err != nil {
		return result, err
	}
	if cmd.RaiseOnNonZeroExit && result.ExitStatus > 0 {
		return result, &FailedError{Name: cmd.name(), ExitStatus: result.ExitStatus, Stdout: result.Stdout, Stderr: result.Stderr}
	}
	if s.backend.LogsExit() {
		s.out.commandExit(cmd, result.ExitStatus, result.Runtime, id)
	}
	return result, nil
}

// PutsByHost prints output under a "<kind> Host: <host>" header, as puts_by_host
// does. Raw output is written as is, without the header.
func PutsByHost(out func(string), host, output, kind string, quiet, raw bool) {
	if raw {
		out(output)
		return
	}
	text := output + "\n\n"
	if !quiet {
		text = kind + " Host: " + host + "\n" + text
	}
	out(text)
}
