package output

// Ports test/output/file_logger_test.rb at Kamal 2.12.0.

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

func newTestFileLogger() (*FileLogger, *fakeFiles, *bytes.Buffer) {
	files, stdout := newFakeFiles(), &bytes.Buffer{}
	return NewFileLogger("log", files, fixedNow, stdout), files, stdout
}

func content(files *fakeFiles) string { return files.files[files.paths()[0]].String() }

func TestFileLoggerDiscardsLinesBeforeStart(t *testing.T) {
	logger, files, _ := newTestFileLogger()
	logger.Write(Entry{Text: "before modify\n"})
	if len(files.paths()) != 0 {
		t.Fatalf("files = %v, want none", files.paths())
	}
}

func TestFileLoggerNamesTheFile(t *testing.T) {
	cases := map[string]Payload{
		"log/2026-11-11T10-30-00_deploy.log":           {Command: "deploy"},
		"log/2026-11-11T10-30-00_app_boot.log":         {Command: "app", Subcommand: "boot"},
		"log/2026-11-11T10-30-00_staging_deploy.log":   {Command: "deploy", Destination: "staging"},
		"log/2026-11-11T10-30-00_staging_app_boot.log": {Command: "app", Subcommand: "boot", Destination: "staging"},
	}
	for want, payload := range cases {
		logger, files, _ := newTestFileLogger()
		if err := logger.Start(payload); err != nil {
			t.Fatal(err)
		}
		if paths := files.paths(); len(paths) != 1 || paths[0] != want {
			t.Errorf("files = %v, want %s", paths, want)
		}
	}
}

func TestFileLoggerWritesLinesToFileAfterStart(t *testing.T) {
	logger, files, _ := newTestFileLogger()
	_ = logger.Start(Payload{Command: "deploy"})
	logger.Write(Entry{Text: "hello\n"})
	logger.Write(Entry{Text: "world\n"})
	if got := content(files); got != "hello\nworld\n" {
		t.Fatalf("content = %q", got)
	}
}

func TestFileLoggerFinishWritesCompletionMessage(t *testing.T) {
	logger, files, stdout := newTestFileLogger()
	_ = logger.Start(Payload{Command: "deploy"})
	logger.Finish(Payload{}, 12)
	if !regexp.MustCompile(`# Completed in \d+\.\d+s`).MatchString(content(files)) || !files.files[files.paths()[0]].closed {
		t.Fatalf("content = %q", content(files))
	}
	if !regexp.MustCompile(`Logs written to .*_deploy\.log`).MatchString(stdout.String()) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestFileLoggerFinishWritesFailureMessageOnException(t *testing.T) {
	logger, files, _ := newTestFileLogger()
	_ = logger.Start(Payload{Command: "deploy"})
	logger.Finish(Payload{Exception: &Exception{Class: "RuntimeError", Message: "boom"}}, 0.4)
	if !strings.Contains(content(files), "# FAILED: RuntimeError: boom (0.4s)") {
		t.Fatalf("content = %q", content(files))
	}
}

func TestFileLoggerCloseIsIdempotent(t *testing.T) {
	logger, _, _ := newTestFileLogger()
	_ = logger.Start(Payload{Command: "deploy"})
	logger.Close()
	logger.Close()
}
