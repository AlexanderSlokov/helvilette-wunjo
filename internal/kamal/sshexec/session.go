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

// Upload is what Session.Upload sends: a local file, or content held in memory.
type Upload struct {
	LocalPath string
	Content   []byte
	InMemory  bool
	Mode      string
}

// Session runs commands on one host, as an SSHKit backend inside an `on` block.
type Session struct {
	Host    string
	backend Backend
	out     *Formatter
	newID   func() string
	env     []EnvVar
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

// Capture runs cmd at Debug and returns its stripped output, as capture does.
func (s *Session) Capture(cmd Command) (string, error) {
	cmd.Verbosity = Debug
	result, err := s.run(cmd)
	return strings.TrimSpace(result.Stdout), err
}

// CaptureWithInfo runs cmd at Info and returns its stripped output.
func (s *Session) CaptureWithInfo(cmd Command) (string, error) {
	cmd.Verbosity = Info
	result, err := s.run(cmd)
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
	if !quiet {
		out(kind + " Host: " + host + "\n")
	}
	out(output + "\n\n")
}
