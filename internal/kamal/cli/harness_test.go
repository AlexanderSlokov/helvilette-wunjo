package cli

// The harness of the ported CLI tests: test/cli/cli_test_case.rb at Kamal 2.12.0.
// fakeHosts stands for SSHKit's printer backend and for the Mocha stubs that the Ruby
// tests put on it.

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/commander"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/commands"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/git"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/secrets"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/sshexec"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/testsupport"
)

// stub replaces matching commands, as a Mocha stub on SSHKit's execute or capture:
// the command is not printed, and it returns Output or fails with Fail.
type stub struct {
	match  func(command string) bool
	output string
	fail   string
	// only limits the stub to captures ("capture") or executes ("execute").
	only string
}

func (s stub) applies(cmd sshexec.Command) bool {
	switch s.only {
	case "capture":
		return cmd.Captures
	case "execute":
		return !cmd.Captures
	default:
		return true
	}
}

// fakeHosts prints each unstubbed command as SSHKit's printer does and records every
// command with its host.
type fakeHosts struct {
	mu       sync.Mutex
	stubs    []stub
	executed []string
	printer  sshexec.Printer
}

func (f *fakeHosts) Run(host string, cmd sshexec.Command, id string, out *sshexec.Formatter) (sshexec.Result, error) {
	text := cmd.ToCommand()
	f.mu.Lock()
	f.executed = append(f.executed, host+": "+text)
	stubs := append([]stub(nil), f.stubs...)
	f.mu.Unlock()
	for _, s := range stubs {
		if s.applies(cmd) && s.match(text) {
			return stubResult(cmd, s)
		}
	}
	return f.printer.Run(host, cmd, id, out)
}

func stubResult(cmd sshexec.Command, s stub) (sshexec.Result, error) {
	if s.fail != "" {
		return sshexec.Result{}, &sshexec.FailedError{Message: s.fail}
	}
	return sshexec.Result{Stdout: s.output, Completed: true}, nil
}

func (f *fakeHosts) Upload(host string, content sshexec.Upload, remote string, out *sshexec.Formatter) error {
	return f.printer.Upload(host, content, remote, out)
}

func (f *fakeHosts) LogsExit() bool       { return false }
func (f *fakeHosts) AlwaysSucceeds() bool { return true }

// world is the setup of one CLI test.
type world struct {
	t             *testing.T
	files         testsupport.FakeFiles
	env           testsupport.FakeEnv
	shell         map[string]localshell.Result
	hosts         *fakeHosts
	local         *fakeHosts
	out           syncBuffer
	errOut        syncBuffer
	execs         []string
	invokes       []string
	recordInvokes bool
}

func newWorld(t *testing.T) *world {
	w := &world{t: t, files: testsupport.FakeFiles{"/k/Dockerfile": ""}, hosts: &fakeHosts{}, local: &fakeHosts{},
		env: testsupport.FakeEnv{"VERSION": "999", "RAILS_MASTER_KEY": "123", "MYSQL_ROOT_PASSWORD": "secret123",
			testsupport.RemoteArchVariable: testsupport.RemoteArch},
		shell: map[string]localshell.Result{
			"docker info --format '{{.Architecture}}'": {Stdout: "x86_64\n", Success: true},
			"git config user.email":                    {Stdout: "deployer@example.com\n", Success: true},
			"git rev-parse":                            {Success: false},
		}}
	print := func(s string) { w.out.WriteString(s) }
	w.hosts.printer.Print, w.local.printer.Print = print, print
	w.local.printer.User = "deployer"
	return w
}

// fixture stores test/fixtures/<name>.yml at the path the Ruby tests use.
func (w *world) fixture(name string) *world {
	content, err := testsupport.ReadFixture(name + ".yml")
	if err != nil {
		w.t.Fatal(err)
	}
	w.files["/k/test/fixtures/"+name+".yml"] = content
	return w
}

// hooksExist makes every hook file exist, as stubbing hook_exists? to true does.
func (w *world) hooksExist() *world {
	for _, hook := range []string{"pre-connect", "pre-build", "pre-deploy", "post-deploy", "pre-app-boot", "post-app-boot",
		"pre-proxy-reboot", "post-proxy-reboot", "docker-setup", "pre-app-remove", "post-app-remove"} {
		w.files[".kamal/hooks/"+hook] = ""
	}
	return w
}

func (w *world) stubRemote(match func(string) bool, output, fail string) *world {
	w.hosts.stubs = append(w.hosts.stubs, stub{match: match, output: output, fail: fail})
	return w
}

// stubCapture stubs only captures, as stubbing capture_with_info does.
func (w *world) stubCapture(match func(string) bool, output string) *world {
	w.hosts.stubs = append(w.hosts.stubs, stub{match: match, output: output, only: "capture"})
	return w
}

func (w *world) stubLocal(match func(string) bool, output, fail string) *world {
	w.local.stubs = append(w.local.stubs, stub{match: match, output: output, fail: fail})
	return w
}

func contains(part string) func(string) bool {
	return func(s string) bool { return strings.Contains(s, part) }
}
func equals(text string) func(string) bool { return func(s string) bool { return s == text } }
func anything(string) bool                 { return true }

func (w *world) newCLI() *CLI {
	sh := &localshell.Fake{Results: w.shell}
	env := configuration.Environment{Lookup: w.env.Lookup, Shell: sh, Git: git.Git{Shell: sh}, Files: w.files, SecretFiles: w.files,
		Substitution: secrets.Substitution{Shell: sh, Lookup: w.env.Lookup}, WorkDir: "/k", TempDir: "/tmp",
		RandomHex: func() string { return "0123456789abcdef" }}
	runtime := commands.Runtime{Now: func() time.Time { return recordedAt }, Performer: "deployer@example.com", Git: git.Git{Shell: sh},
		RandomHex: func(n int) string { return strings.Repeat("a1", n) }, Files: w.files, WorkDir: "/k"}
	c := New(commander.New(env, runtime, false), w.runtime())
	if w.recordInvokes {
		c.invoke = func(name string, args []string, opts Options) error {
			w.invokes = append(w.invokes, strings.TrimSpace(name+" "+strings.Join(args, " ")+" | "+opts.Describe()))
			return nil
		}
	}
	return c
}

var recordedAt = time.Date(2026, 11, 11, 10, 30, 0, 0, time.UTC)

func (w *world) runtime() Runtime {
	return Runtime{Stdout: &w.out, Stderr: &w.errOut, Stdin: strings.NewReader(""),
		Remote: func(*configuration.Configuration) sshexec.Backend { return w.hosts }, Local: w.local,
		Exec:    func(command string) error { w.execs = append(w.execs, command); return nil },
		Forward: func([]string, int, sshexec.Options) (func(), error) { return func() {}, nil },
		Sleep:   func(time.Duration) {}, Now: func() time.Time { return recordedAt },
		RandomHex: func(n int) string { return strings.Repeat("a1", n) }, Files: fakeLocalFiles{w.files},
		Getenv: func(name string) (string, bool) {
			if name == "PWD" {
				return "/k", true
			}
			return w.env.Lookup(name)
		},
		Adapters: func(name string) (SecretsAdapter, error) { return nil, errors.New("no adapters in tests") },
	}
}

// run runs the command line and returns the output without surrounding whitespace,
// as the Ruby tests' stdouted does. It fails the test when the command fails.
func (w *world) run(args ...string) string {
	w.t.Helper()
	status := w.newCLI().Run(args)
	if status != 0 {
		w.t.Fatalf("wunjo %s exited %d:\n%s", strings.Join(args, " "), status, w.out.String())
	}
	return strings.TrimSpace(w.out.String())
}

// runFailing runs the command line and returns the output of a failed run.
func (w *world) runFailing(args ...string) string {
	w.t.Helper()
	if status := w.newCLI().Run(args); status == 0 {
		w.t.Fatalf("wunjo %s succeeded, want a failure:\n%s", strings.Join(args, " "), w.out.String())
	}
	return w.out.String()
}

// syncBuffer is a buffer that parallel host sessions can write to at once.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) WriteString(s string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.WriteString(s)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// fakeLocalFiles adds writes and globs to the fake files.
type fakeLocalFiles struct{ testsupport.FakeFiles }

func (f fakeLocalFiles) WriteFile(path string, content []byte, _ uint32) error {
	f.FakeFiles[path] = string(content)
	return nil
}

func (f fakeLocalFiles) MkdirAll(path string) error {
	f.FakeFiles[path] = ""
	return nil
}

func (f fakeLocalFiles) RemoveAll(path string) error {
	delete(f.FakeFiles, path)
	return nil
}

func (f fakeLocalFiles) Glob(pattern string) ([]string, error) {
	re := regexp.MustCompile("^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), `\?`, ".") + "$")
	var matches []string
	for path := range f.FakeFiles {
		if re.MatchString(path) {
			matches = append(matches, path)
		}
	}
	return matches, nil
}

func assertContains(t *testing.T, output, want string) {
	t.Helper()
	if !strings.Contains(output, want) {
		t.Fatalf("output lacks %q:\n%s", want, output)
	}
}

func assertNotContains(t *testing.T, output, unwanted string) {
	t.Helper()
	if strings.Contains(output, unwanted) {
		t.Fatalf("output has %q:\n%s", unwanted, output)
	}
}

func assertMatches(t *testing.T, output, pattern string) {
	t.Helper()
	if !regexp.MustCompile(pattern).MatchString(output) {
		t.Fatalf("output does not match %s:\n%s", pattern, output)
	}
}

func regexpMatch(pattern, output string) bool { return regexp.MustCompile(pattern).MatchString(output) }

// assertHookRan checks the hook ran count times, as assert_hook_ran does.
func assertHookRan(t *testing.T, output, hook string, count int) {
	t.Helper()
	parts := make([]string, count)
	for i := range parts {
		parts[i] = regexp.QuoteMeta("/usr/bin/env .kamal/hooks/" + hook)
	}
	assertMatches(t, output, "(?s)"+strings.Join(parts, ".*"))
}
