// Package output writes the log of each modifying command to the destinations that
// the output section of the deploy file names: a file and an OpenTelemetry (OTel)
// collector.
//
// Ports lib/kamal/output/base_logger.rb at Kamal 2.12.0 and the parts of
// ActiveSupport::BroadcastLogger and ActiveSupport::Notifications that Kamal uses.
package output

import (
	"math"
	"sync"
	"time"
)

// Severity is a Ruby Logger level: DebugSeverity is Logger::DEBUG.
type Severity int

// The Logger levels, and NoSeverity for a line without one, such as a line that
// `say` prints.
const (
	NoSeverity Severity = iota - 1
	DebugSeverity
	InfoSeverity
	WarnSeverity
	ErrorSeverity
	FatalSeverity
)

// Payload describes one modifying command, as the "modify.kamal" notification does.
type Payload struct {
	Command     string
	Subcommand  string
	Destination string
	// Exception is set when the command fails.
	Exception *Exception
}

// Exception is the Ruby class name and the message of a failure, such as
// {"RuntimeError", "boom"}.
type Exception struct {
	Class   string
	Message string
}

// Entry is one line of output with the context that the formatter knows: the host
// and stream of the command, and the level of the line.
type Entry struct {
	Text     string
	Host     string
	IOStream string
	Severity Severity
}

// Logger is one destination. Start and Finish surround a modifying command, and
// runtime is in seconds, rounded to a tenth.
type Logger interface {
	Start(p Payload) error
	Finish(p Payload, runtime float64)
	Write(e Entry)
	Close()
}

// Broadcast sends each line and event to every logger, and times each command.
// After Close, loggers receive lines but no more events, as Kamal's loggers
// unsubscribe from the notification when they close.
//
//	logs := output.NewBroadcast(time.Now)
//	logs.Add(fileLogger)
type Broadcast struct {
	mu           sync.Mutex
	loggers      []Logger
	now          func() time.Time
	started      time.Time
	unsubscribed bool
}

// NewBroadcast returns a Broadcast with no loggers that reads the time from now.
func NewBroadcast(now func() time.Time) *Broadcast { return &Broadcast{now: now} }

// Add adds a logger.
func (b *Broadcast) Add(logger Logger) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.loggers = append(b.loggers, logger)
}

// Start starts the clock and tells each logger that a command starts.
func (b *Broadcast) Start(p Payload) error {
	loggers, ok := b.subscribers()
	if !ok {
		return nil
	}
	b.mu.Lock()
	b.started = b.now()
	b.mu.Unlock()
	for _, logger := range loggers {
		if err := logger.Start(p); err != nil {
			return err
		}
	}
	return nil
}

// Finish tells each logger that the command ended, with its runtime.
func (b *Broadcast) Finish(p Payload) {
	loggers, ok := b.subscribers()
	if !ok {
		return
	}
	b.mu.Lock()
	runtime := math.Round(b.now().Sub(b.started).Seconds()*10) / 10
	b.mu.Unlock()
	for _, logger := range loggers {
		logger.Finish(p, runtime)
	}
}

// Write sends one line to each logger.
func (b *Broadcast) Write(e Entry) {
	for _, logger := range b.all() {
		logger.Write(e)
	}
}

// Close closes each logger. Kamal closes after the outermost modifying command
// and again at exit, so Close may run more than once.
func (b *Broadcast) Close() {
	b.mu.Lock()
	b.unsubscribed = true
	b.mu.Unlock()
	for _, logger := range b.all() {
		logger.Close()
	}
}

func (b *Broadcast) all() []Logger {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]Logger(nil), b.loggers...)
}

func (b *Broadcast) subscribers() ([]Logger, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]Logger(nil), b.loggers...), !b.unsubscribed
}
