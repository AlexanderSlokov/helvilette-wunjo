package sshexec

import (
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Process is one local program to start.
type Process struct {
	Name   string
	Args   []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// ProcessRunner starts programs. Tests pass a fake that records them.
type ProcessRunner interface {
	RunProcess(p Process) (exitStatus int, err error)
}

// OSProcesses starts programs with os/exec.
type OSProcesses struct{}

// RunProcess runs p and returns its exit status. An error means the program could
// not start.
func (OSProcesses) RunProcess(p Process) (int, error) {
	cmd := exec.Command(p.Name, p.Args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = p.Stdin, p.Stdout, p.Stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	if err != nil {
		return -1, err
	}
	return 0, nil
}

// streamLogger collects a stream and logs each chunk at Debug, as SSHKit logs
// command data while it arrives.
type streamLogger struct {
	mu     sync.Mutex
	buf    strings.Builder
	stream string
	id     string
	out    *Formatter
}

func (s *streamLogger) Write(p []byte) (int, error) {
	s.mu.Lock()
	s.buf.Write(p)
	s.mu.Unlock()
	s.out.commandData(s.stream, string(p), s.id)
	return len(p), nil
}

func (s *streamLogger) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// runLogged starts a process with logged output and returns its result.
func runLogged(runner ProcessRunner, p Process, id string, out *Formatter) (Result, error) {
	stdout := &streamLogger{stream: "stdout", id: id, out: out}
	stderr := &streamLogger{stream: "stderr", id: id, out: out}
	p.Stdout, p.Stderr = stdout, stderr
	started := time.Now()
	status, err := runner.RunProcess(p)
	return Result{Stdout: stdout.String(), Stderr: stderr.String(), ExitStatus: status, Runtime: time.Since(started)}, err
}

// Local runs commands on this machine with /bin/sh, as SSHKit's local backend does.
type Local struct {
	Processes ProcessRunner
}

// Run logs and runs cmd through /bin/sh -c.
func (l Local) Run(host string, cmd Command, id string, out *Formatter) (Result, error) {
	out.commandStart(cmd, host, id)
	return runLogged(l.Processes, Process{Name: "/bin/sh", Args: []string{"-c", cmd.ToCommand()}}, id, out)
}

// Upload writes the content to remote on this machine.
func (l Local) Upload(host string, content Upload, remote string, out *Formatter) error {
	body, err := uploadBody(content)
	if err != nil {
		return err
	}
	status, err := l.Processes.RunProcess(Process{Name: "/bin/sh", Args: []string{"-c", "cat > " + shellQuote(remote)}, Stdin: body})
	return statusError("upload to "+remote, status, err)
}

// LogsExit is true: the local backend logs each exit.
func (l Local) LogsExit() bool { return true }

// AlwaysSucceeds is false.
func (l Local) AlwaysSucceeds() bool { return false }

// LocalHost is the host name that SSHKit shows for local commands.
const LocalHost = "localhost"
