package sshexec

// Ports SSHKit::Formatter::Pretty and SSHKit::Color from SSHKit 1.25.1.

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

var levelNames = []string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}

// ANSI codes of SSHKit::Color for the colors that the pretty formatter uses.
var colorCodes = map[string]string{"black": "30", "red": "31", "green": "32", "yellow": "33", "blue": "34"}

var levelColors = []string{"black", "blue", "yellow", "red", "red"}

// Formatter writes log lines in SSHKit's pretty format:
//
//	INFO [1a2b3c4d] Running docker ps on 1.1.1.1
//
// Lines below Verbosity are dropped. Color is on when the output is a terminal.
type Formatter struct {
	mu        sync.Mutex
	out       io.Writer
	Verbosity Level
	Color     bool
	// Sink receives every line at every level, as Kamal's output formatter sends
	// each message to the output loggers. It runs in the order of the lines.
	Sink func(LogLine)
}

// LogLine is one formatted line, with its newline, and the context of the command
// that wrote it: the host, and "stdout" or "stderr" for command output.
type LogLine struct {
	Text   string
	Level  Level
	Host   string
	Stream string
}

// lineContext is the host and stream of a line, empty outside a command.
type lineContext struct{ host, stream string }

// NewFormatter returns a formatter at Info that writes to out.
func NewFormatter(out io.Writer, color bool) *Formatter {
	return &Formatter{out: out, Verbosity: Info, Color: color}
}

// Log writes message at level without a command ID.
func (f *Formatter) Log(level Level, message string) { f.write(level, message, "", lineContext{}) }

func (f *Formatter) colorize(text, color string, bold bool) string {
	if !f.Color {
		return text
	}
	code := colorCodes[color]
	if bold {
		code = "1;" + code
	}
	return "\033[" + code + "m" + text + "\033[0m"
}

func (f *Formatter) write(level Level, message, id string, ctx lineContext) {
	if id != "" {
		message = "[" + f.colorize(id, "green", false) + "] " + message
	}
	name := fmt.Sprintf("%6s", levelNames[level])
	line := f.colorize(name, levelColors[level], false) + " " + message + "\n"
	f.mu.Lock()
	defer f.mu.Unlock()
	if level >= f.Verbosity {
		io.WriteString(f.out, line)
	}
	if f.Sink != nil {
		f.Sink(LogLine{Text: line, Level: level, Host: ctx.host, Stream: ctx.stream})
	}
}

// commandStart logs "Running <command> on <host>" and, at Debug, the full command.
// A host with a user, as SSHKit's local host has, shows "as <user>@<host>".
func (f *Formatter) commandStart(cmd Command, host, user, id string) {
	where := "on " + f.colorize(host, "blue", false)
	if user != "" {
		where = "as " + f.colorize(user, "blue", false) + "@" + f.colorize(host, "blue", false)
	}
	message := "Running " + f.colorize(cmd.Redacted(), "yellow", true) + " " + where
	f.write(cmd.Verbosity, message, id, lineContext{host: host})
	f.write(Debug, "Command: "+f.colorize(cmd.ToCommand(), "blue", false), id, lineContext{host: host})
}

// commandData logs one chunk of output at Debug, green for stdout and red for stderr.
func (f *Formatter) commandData(host, stream, data, id string) {
	color := "green"
	if stream == "stderr" {
		color = "red"
	}
	f.write(Debug, f.colorize(strings.TrimSuffix("\t"+data, "\n"), color, false), id, lineContext{host: host, stream: stream})
}

// commandExit logs the runtime and exit status.
func (f *Formatter) commandExit(cmd Command, host string, status int, runtime time.Duration, id string) {
	result := f.colorize("successful", "green", true)
	if status > 0 {
		result = f.colorize("failed", "red", true)
	}
	message := fmt.Sprintf("Finished in %5.3f seconds with exit status %d (%s).", runtime.Seconds(), status, result)
	f.write(cmd.Verbosity, message, id, lineContext{host: host})
}
