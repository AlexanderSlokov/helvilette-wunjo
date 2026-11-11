package cli

// Ports test/cli/modify_test.rb at Kamal 2.12.0, and runs a modifying command with
// a file logger and an OTel logger in the output section.

import (
	"errors"
	"io"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/commander"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/output"
)

// fakeLogFiles keeps the files of the file logger in memory.
type fakeLogFiles struct {
	mu    sync.Mutex
	files map[string]*strings.Builder
}

func (f *fakeLogFiles) MkdirAll(string) error { return nil }

func (f *fakeLogFiles) OpenAppend(path string) (io.WriteCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.files == nil {
		f.files = map[string]*strings.Builder{}
	}
	if f.files[path] == nil {
		f.files[path] = &strings.Builder{}
	}
	return logFile{f, path}, nil
}

// only returns the path and content of the one file written.
func (f *fakeLogFiles) only(t *testing.T) (string, string) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.files) != 1 {
		t.Fatalf("log files = %d, want 1", len(f.files))
	}
	for path, content := range f.files {
		return path, content.String()
	}
	return "", ""
}

type logFile struct {
	files *fakeLogFiles
	path  string
}

func (l logFile) Write(p []byte) (int, error) {
	l.files.mu.Lock()
	defer l.files.mu.Unlock()
	return l.files.files[l.path].Write(p)
}

func (l logFile) Close() error { return nil }

// fakeOTelPoster records each body posted to the collector.
type fakeOTelPoster struct {
	mu     sync.Mutex
	urls   []string
	bodies []string
}

func (p *fakeOTelPoster) Post(url string, body []byte) (int, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.urls, p.bodies = append(p.urls, url), append(p.bodies, string(body))
	return 200, "OK", nil
}

// recordingOutputLogger counts the events and closes that it receives.
type recordingOutputLogger struct{ starts, finishes, closes int }

func (r *recordingOutputLogger) Start(output.Payload) error     { r.starts++; return nil }
func (r *recordingOutputLogger) Finish(output.Payload, float64) { r.finishes++ }
func (r *recordingOutputLogger) Write(output.Entry)             {}
func (r *recordingOutputLogger) Close()                         { r.closes++ }

// modifyCLI returns a CLI configured with deploy_simple.yml and a recording logger,
// as run_modify builds Kamal::Cli::Main for `deploy`.
func modifyCLI(t *testing.T) (*CLI, *recordingOutputLogger) {
	c := newWorld(t).fixture("deploy_simple").newCLI()
	c.kamal.Configure(commander.ConfigOptions{ConfigFile: "/k/test/fixtures/deploy_simple.yml"})
	c.first = []string{"deploy"}
	logger := &recordingOutputLogger{}
	c.logs.Add(logger)
	return c, logger
}

func TestModifyEnablesLogging(t *testing.T) {
	c, _ := modifyCLI(t)
	if c.logging {
		t.Fatal("logging before modify")
	}
	_ = c.modify(false, func() error { return nil })
	if !c.logging {
		t.Fatal("no logging after modify")
	}
}

func TestNestedModifyOnlyInstrumentsOutermostAndClosesOnce(t *testing.T) {
	c, logger := modifyCLI(t)
	_ = c.modify(false, func() error { return c.modify(false, func() error { return nil }) })
	if logger.starts != 1 || logger.finishes != 1 || logger.closes != 1 {
		t.Fatalf("starts %d, finishes %d, closes %d, want 1 each", logger.starts, logger.finishes, logger.closes)
	}
}

func TestModifyDepthResetsAfterCompletionAndError(t *testing.T) {
	c, _ := modifyCLI(t)
	_ = c.modify(false, func() error { return nil })
	if err := c.modify(false, func() error { return errors.New("boom") }); err == nil || c.modifyDepth != 0 {
		t.Fatalf("err = %v, depth = %d", err, c.modifyDepth)
	}
}

const outputConfig = "/k/test/fixtures/deploy_with_output.yml"

// outputWorld writes deploy_simple.yml with the output section.
func outputWorld(t *testing.T, section string) *world {
	w := newWorld(t).fixture("deploy_simple")
	w.files[outputConfig] = w.files["/k/test/fixtures/deploy_simple.yml"] + "\noutput:\n" + section
	return w
}

func TestFileLoggerWritesTheLogOfAModifyingCommand(t *testing.T) {
	w := outputWorld(t, "  file:\n    path: log/kamal\n")
	stdout := w.run("prune", "all", "-c", outputConfig)
	path, content := w.logFiles.only(t)
	if path != "log/kamal/2026-11-11T10-30-00_prune_all.log" {
		t.Fatalf("path = %s", path)
	}
	for _, want := range []string{"Acquiring the deploy lock...\n", "  INFO [a1a1a1a1] Running docker image prune --force --filter label=service=app on 1.1.1.1\n",
		" DEBUG [a1a1a1a1] Command: docker image prune", "# Completed in 0.0s\n"} {
		assertContains(t, content, want)
	}
	assertContains(t, stdout, "Logs written to log/kamal/2026-11-11T10-30-00_prune_all.log")
}

func TestFileLoggerRecordsTheFailure(t *testing.T) {
	w := outputWorld(t, "  file:\n    path: log/kamal\n").stubRemote(contains("docker image prune --force"), "", "boom")
	w.runFailing("prune", "all", "-c", outputConfig)
	_, content := w.logFiles.only(t)
	assertContains(t, content, "# FAILED: SSHKit::Runner::ExecuteError: Exceptions on 2 hosts:\nException while executing on host 1.1.1.")
}

func TestOtelLoggerShipsEventsAndLinesAtExit(t *testing.T) {
	w := outputWorld(t, "  otel:\n    endpoint: http://collector:4318\n")
	stdout := w.run("prune", "all", "-c", outputConfig)
	assertContains(t, stdout, "Logs sent to http://collector:4318")
	shipped := strings.Join(w.otel.bodies, "\n")
	for _, want := range []string{`"eventName":"kamal.start"`, `"stringValue":"prune all"`, `"eventName":"kamal.complete"`,
		`"key":"server.address","value":{"stringValue":"1.1.1.1"}`, `"key":"kamal.performer","value":{"stringValue":"deployer@example.com"}`} {
		assertContains(t, shipped, want)
	}
	urls := slices.Compact(sorted(w.otel.urls))
	if len(urls) != 1 || urls[0] != "http://collector:4318/v1/logs" {
		t.Fatalf("urls = %v", urls)
	}
}

func sorted(items []string) []string {
	out := append([]string(nil), items...)
	sort.Strings(out)
	return out
}

// Kamal 2.12.0 wraps deploy, redeploy, and rollback in an unlocked modify, so their
// log starts before the build. The integration suite found the missing wrapper.
func TestDeployLogStartsBeforeTheBuild(t *testing.T) {
	for _, command := range []string{"deploy", "redeploy"} {
		w := outputWorld(t, "  file:\n    path: log/kamal\n")
		w.recordInvokes = true
		w.run(command, "-c", outputConfig)
		path, content := w.logFiles.only(t)
		if !strings.HasSuffix(path, "_"+command+".log") {
			t.Fatalf("log file = %s", path)
		}
		assertContains(t, content, "Build and push app image...\n")
		assertContains(t, content, "Acquiring the deploy lock...\n")
	}
}
