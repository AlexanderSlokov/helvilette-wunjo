package output

// Ports lib/kamal/output/file_logger.rb at Kamal 2.12.0.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Files is the file system that FileLogger writes to.
type Files interface {
	MkdirAll(path string) error
	OpenAppend(path string) (io.WriteCloser, error)
}

// OSFiles writes to the local file system.
type OSFiles struct{}

// MkdirAll creates path and its parents.
func (OSFiles) MkdirAll(path string) error { return os.MkdirAll(path, 0o755) }

// OpenAppend opens path for appending and creates it when it is missing.
func (OSFiles) OpenAppend(path string) (io.WriteCloser, error) {
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
}

// FileLogger writes the lines of each modifying command to its own file in a
// directory, such as log/kamal/2026-11-11T10-30-00_staging_app_boot.log. Lines
// outside a command go nowhere.
type FileLogger struct {
	dir    string
	files  Files
	now    func() time.Time
	stdout io.Writer

	mu       sync.Mutex
	file     io.WriteCloser
	filePath string
}

// NewFileLogger returns a logger that writes into dir and prints the file path to
// stdout when a command ends.
//
//	logger := output.NewFileLogger("log/kamal", output.OSFiles{}, time.Now, os.Stdout)
func NewFileLogger(dir string, files Files, now func() time.Time, stdout io.Writer) *FileLogger {
	return &FileLogger{dir: dir, files: files, now: now, stdout: stdout}
}

// Start opens the log file of the command.
func (l *FileLogger) Start(p Payload) error {
	if err := l.files.MkdirAll(l.dir); err != nil {
		return err
	}
	path := filepath.Join(l.dir, l.filename(p))
	file, err := l.files.OpenAppend(path)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.file, l.filePath = file, path
	return nil
}

// Finish writes the result line, closes the file, and prints its path.
func (l *FileLogger) Finish(p Payload, runtime float64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}
	if p.Exception != nil {
		fmt.Fprintf(l.file, "# FAILED: %s: %s (%ss)\n", p.Exception.Class, p.Exception.Message, rubyFloat(runtime))
	} else {
		fmt.Fprintf(l.file, "# Completed in %ss\n", rubyFloat(runtime))
	}
	l.file.Close()
	l.file = nil
	fmt.Fprintf(l.stdout, "Logs written to %s\n", l.filePath)
}

// Write appends a line while a command runs.
func (l *FileLogger) Write(e Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		io.WriteString(l.file, e.Text)
	}
}

// Close closes an open file. It is safe to call more than once.
func (l *FileLogger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
}

// filename joins the local time, the destination, and the command, such as
// "2026-11-11T10-30-00_staging_app_boot.log".
func (l *FileLogger) filename(p Payload) string {
	command := strings.Join(present(p.Command, p.Subcommand), "_")
	parts := present(l.now().Format("2006-01-02T15-04-05"), p.Destination, command)
	return strings.Join(parts, "_") + ".log"
}

// present drops empty strings, as Ruby's compact drops nil.
func present(parts ...string) []string {
	var out []string
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// rubyFloat writes a float as Ruby's Float#to_s does for a runtime: 12 is "12.0".
func rubyFloat(f float64) string {
	text := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(text, ".") {
		text += ".0"
	}
	return text
}
