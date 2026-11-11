package sshexec

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
)

func cmdOf(words ...string) Command { return NewCommand(shell.Words(words...)) }

func TestCommandMapsEveryNameButDocker(t *testing.T) {
	cases := map[string]Command{
		"docker ps":                      cmdOf("docker", "ps"),
		"/usr/bin/env mkdir -p .kamal":   cmdOf("mkdir", "-p", ".kamal"),
		`[ "${EUID:-$(id -u)}" -eq 0 ]`:  cmdOf(`[ "${EUID:-$(id -u)}" -eq 0 ]`),
		"/usr/bin/env sh -c 'echo a; b'": cmdOf("sh", "-c", "'echo a; b'"),
		"kill -HUP $PPID":                cmdOf("kill -HUP $PPID"),
		"line one; line two":             cmdOf("line one\n  line two\n"),
	}
	for want, cmd := range cases {
		if got := cmd.ToCommand(); got != want {
			t.Errorf("ToCommand = %q, want %q", got, want)
		}
	}
}

func TestCommandEnvWrapsInExport(t *testing.T) {
	cmd := cmdOf("docker", "buildx", "build")
	cmd.Env = []EnvVar{{"BUILDKIT_NO_CLIENT_TOKEN", "1"}, {"QUOTE", `a"b`}}
	want := `( export BUILDKIT_NO_CLIENT_TOKEN="1" QUOTE="a\"b" ; docker buildx build )`
	if got := cmd.ToCommand(); got != want {
		t.Fatalf("got %s", got)
	}
}

func TestCommandRedaction(t *testing.T) {
	cmd := NewCommand(shell.Line{shell.Plain("docker"), shell.Plain("login"), shell.Plain("-p"), shell.Sensitive(`"secret"`, "[REDACTED]")})
	if cmd.Redacted() != "docker login -p [REDACTED]" || cmd.ToCommand() != `docker login -p "secret"` {
		t.Fatalf("redacted %q command %q", cmd.Redacted(), cmd.ToCommand())
	}
}

func TestFailedErrorMessage(t *testing.T) {
	err := &FailedError{Name: "docker", ExitStatus: 1, Stderr: "boom\n"}
	want := "docker exit status: 1\ndocker stdout: Nothing written\ndocker stderr: boom\n"
	if err.Error() != want {
		t.Fatalf("got %q", err.Error())
	}
}

func TestFormatterPrettyLines(t *testing.T) {
	var out bytes.Buffer
	f := NewFormatter(&out, false)
	f.commandStart(cmdOf("docker", "ps"), "1.1.1.1", "1a2b3c4d")
	f.commandExit(cmdOf("docker", "ps"), 0, 1500*time.Millisecond, "1a2b3c4d")
	f.Log(Debug, "hidden")
	want := "  INFO [1a2b3c4d] Running docker ps on 1.1.1.1\n  INFO [1a2b3c4d] Finished in 1.500 seconds with exit status 0 (successful).\n"
	if out.String() != want {
		t.Fatalf("got\n%s", out.String())
	}
}

// scriptedBackend returns a fixed result for each rendered command and records them.
type scriptedBackend struct {
	results map[string]Result
	ran     []string
}

func (b *scriptedBackend) Run(host string, cmd Command, id string, out *Formatter) (Result, error) {
	b.ran = append(b.ran, host+": "+cmd.ToCommand())
	return b.results[cmd.ToCommand()], nil
}

func (b *scriptedBackend) Upload(string, Upload, string, *Formatter) error { return nil }
func (b *scriptedBackend) LogsExit() bool                                  { return true }
func (b *scriptedBackend) AlwaysSucceeds() bool                            { return false }

func newRunner(b Backend, out io.Writer) *Runner {
	return &Runner{Backend: b, Out: NewFormatter(out, false), NewID: func() string { return "00000000" }, Sleep: func(time.Duration) {}}
}

func TestSessionExecuteCaptureAndTest(t *testing.T) {
	b := &scriptedBackend{results: map[string]Result{
		"docker ps":          {Stdout: " id1 \n"},
		"/usr/bin/env false": {ExitStatus: 1, Stderr: "no"},
	}}
	err := newRunner(b, io.Discard).On([]string{"1.1.1.1"}, func(s *Session) error {
		if out, err := s.Capture(cmdOf("docker", "ps")); err != nil || out != "id1" {
			t.Errorf("capture = %q, %v", out, err)
		}
		if s.Test(cmdOf("false")) {
			t.Error("test of false passed")
		}
		return s.Execute(cmdOf("false"))
	})
	want := "Exception while executing on host 1.1.1.1: false exit status: 1\nfalse stdout: Nothing written\nfalse stderr: no\n"
	if err == nil || err.Error() != want {
		t.Fatalf("error = %q", err)
	}
	var failed *FailedError
	if !errors.As(err, &failed) {
		t.Fatal("cause is not a FailedError")
	}
}

func TestSessionWithAddsEnv(t *testing.T) {
	b := &scriptedBackend{}
	_ = newRunner(b, io.Discard).On([]string{"h"}, func(s *Session) error {
		return s.With([]EnvVar{{"A", "1"}}, func() error { return s.Execute(cmdOf("docker", "ps")) })
	})
	if len(b.ran) != 1 || b.ran[0] != `h: ( export A="1" ; docker ps )` {
		t.Fatalf("ran %v", b.ran)
	}
}

func TestOnReportsEveryFailedHost(t *testing.T) {
	err := newRunner(&scriptedBackend{}, io.Discard).On([]string{"a", "b", "c"}, func(s *Session) error {
		if s.Host == "b" {
			return nil
		}
		return errors.New("broke")
	})
	want := "Exceptions on 2 hosts:\nException while executing on host a: broke\nException while executing on host c: broke"
	if err == nil || err.Error() != want {
		t.Fatalf("error = %q", err)
	}
}

func TestInGroupsWaitsBetweenGroups(t *testing.T) {
	r := newRunner(&scriptedBackend{}, io.Discard)
	var slept []time.Duration
	r.Sleep = func(d time.Duration) { slept = append(slept, d) }
	var groups [][]string
	var current []string
	_ = r.InGroups([]string{"a", "b", "c"}, 2, time.Second, func(s *Session) error { return nil })
	_ = groups
	_ = current
	if len(slept) != 1 || slept[0] != time.Second {
		t.Fatalf("slept %v", slept)
	}
}

func TestOnRolesSequentialRunsRolesInOrderPerHost(t *testing.T) {
	var ran []string
	roles := []RoleHosts{{"web", []string{"h1"}}, {"workers", []string{"h1", "h2"}}}
	err := newRunner(&scriptedBackend{}, io.Discard).OnRoles(roles, []string{"h1"}, false, func(s *Session, role string) error {
		ran = append(ran, s.Host+":"+role)
		return nil
	})
	if err != nil || strings.Join(ran, ",") != "h1:web,h1:workers" {
		t.Fatalf("ran %v, %v", ran, err)
	}
}

func TestOnRolesParallelNamesTheRole(t *testing.T) {
	roles := []RoleHosts{{"web", []string{"h1"}}}
	err := newRunner(&scriptedBackend{}, io.Discard).OnRoles(roles, []string{"h1"}, true, func(*Session, string) error { return errors.New("x") })
	if err == nil || err.Error() != "Exception while executing on web: Exception while executing on host h1: x" {
		t.Fatalf("error = %v", err)
	}
}

// Ports test/sshkit_dns_retry_test.rb at Kamal 2.12.0.
func TestRetriesDNSErrors(t *testing.T) {
	attempts := 0
	var log bytes.Buffer
	retry := DNSRetry{Retries: 2, Sleep: func(time.Duration) {}}
	err := retry.Do("example.com", NewFormatter(&log, false), func() error {
		attempts++
		if attempts < 3 {
			return &DNSError{Message: "getaddrinfo: Temporary failure in name resolution"}
		}
		return nil
	})
	if err != nil || attempts != 3 || !strings.Contains(log.String(), "Retrying DNS for example.com (attempt 1/2)") {
		t.Fatalf("attempts %d err %v log %s", attempts, err, log.String())
	}
}

func TestDoesNotRetryNonDNSErrors(t *testing.T) {
	attempts := 0
	refused := errors.New("connection refused")
	err := DNSRetry{Retries: 2, Sleep: func(time.Duration) {}}.Do("example.com", NewFormatter(io.Discard, false), func() error {
		attempts++
		return refused
	})
	if !errors.Is(err, refused) || attempts != 1 {
		t.Fatalf("attempts %d err %v", attempts, err)
	}
}

// fakeProcesses scripts ssh runs: the stderr and exit status of each attempt.
type fakeProcesses struct {
	attempts []Result
	calls    [][]string
}

func (f *fakeProcesses) RunProcess(p Process) (int, error) {
	f.calls = append(f.calls, append([]string{p.Name}, p.Args...))
	if len(f.attempts) == 0 {
		return 0, nil
	}
	next := f.attempts[0]
	f.attempts = f.attempts[1:]
	io.WriteString(p.Stdout, next.Stdout)
	io.WriteString(p.Stderr, next.Stderr)
	return next.ExitStatus, nil
}

func TestOpenSSHBackendRetriesDNSErrorsWhenConnecting(t *testing.T) {
	procs := &fakeProcesses{attempts: []Result{
		{ExitStatus: 255, Stderr: "ssh: Could not resolve hostname unknown.example.com: nodename nor servname provided, or not known"},
		{Stdout: "ok\n"},
	}}
	var log bytes.Buffer
	backend := &OpenSSH{Processes: procs, ControlDir: t.TempDir(), DNS: DNSRetry{Retries: 3, Sleep: func(time.Duration) {}}}
	result, err := backend.Run("unknown.example.com", cmdOf("true"), "id", NewFormatter(&log, false))
	if err != nil || result.Stdout != "ok\n" || len(procs.calls) != 2 {
		t.Fatalf("result %+v err %v calls %d", result, err, len(procs.calls))
	}
	if !strings.Contains(log.String(), "Retrying DNS for unknown.example.com") {
		t.Fatalf("log = %s", log.String())
	}
}

func TestOpenSSHArgsFollowTheDeployFile(t *testing.T) {
	yes := true
	procs := &fakeProcesses{}
	backend := &OpenSSH{Processes: procs, ControlDir: t.TempDir(), Options: Options{
		User: "app", Port: "2222", ProxyJump: "root@bastion", Keys: []string{"~/.ssh/deploy"}, KeysOnly: true,
		ForwardAgent: &yes, ConfigFiles: []string{"~/.ssh/a", "~/.ssh/b"}, ControlPersist: 900,
	}}
	if _, err := backend.Run("1.1.1.1", cmdOf("docker", "ps"), "id", NewFormatter(io.Discard, false)); err != nil {
		t.Fatal(err)
	}
	args := strings.Join(procs.calls[0], " ")
	for _, want := range []string{"ControlPersist=900", "-l app", "-p 2222", "-J root@bastion", "-i ~/.ssh/deploy", "IdentitiesOnly=yes", " -A ", "StrictHostKeyChecking=accept-new", "1.1.1.1 docker ps"} {
		if !strings.Contains(args, want) {
			t.Errorf("args lack %q: %s", want, args)
		}
	}
	if !strings.Contains(args, "-F "+backend.ControlDir+"/config") {
		t.Errorf("args lack the generated config: %s", args)
	}
}

func TestOpenSSHTreatsExit255AsConnectionError(t *testing.T) {
	procs := &fakeProcesses{attempts: []Result{{ExitStatus: 255, Stderr: "Connection refused"}}}
	backend := &OpenSSH{Processes: procs, ControlDir: t.TempDir()}
	_, err := backend.Run("1.1.1.1", cmdOf("true"), "id", NewFormatter(io.Discard, false))
	var conn *ConnectionError
	if !errors.As(err, &conn) || conn.Message != "Connection refused" {
		t.Fatalf("error = %v", err)
	}
}

func TestPrinterLogsWithoutRunning(t *testing.T) {
	var out bytes.Buffer
	printer := &Printer{Print: func(s string) { out.WriteString(s) }}
	r := newRunner(printer, &out)
	_ = r.On([]string{"1.1.1.1"}, func(s *Session) error {
		if !s.Test(cmdOf("false")) {
			t.Error("printer test failed")
		}
		_ = s.Upload(Upload{Content: []byte("A=1\n"), InMemory: true}, ".kamal/env")
		return s.Execute(cmdOf("docker", "ps"))
	})
	if !strings.Contains(out.String(), "Uploading \"A=1\\n\" to .kamal/env on 1.1.1.1") || !strings.Contains(out.String(), "Running docker ps on 1.1.1.1") {
		t.Fatalf("output:\n%s", out.String())
	}
}

func TestPutsByHost(t *testing.T) {
	var out strings.Builder
	PutsByHost(func(s string) { out.WriteString(s) }, "1.1.1.1", "output", "App", false, false)
	if out.String() != "App Host: 1.1.1.1\noutput\n\n" {
		t.Fatalf("got %q", out.String())
	}
}

func TestLocalRunsThroughSh(t *testing.T) {
	result, err := Local{Processes: OSProcesses{}}.Run(LocalHost, cmdOf("echo", "hi"), "id", NewFormatter(io.Discard, false))
	if err != nil || result.Stdout != "hi\n" || result.ExitStatus != 0 {
		t.Fatalf("result %+v err %v", result, err)
	}
}
