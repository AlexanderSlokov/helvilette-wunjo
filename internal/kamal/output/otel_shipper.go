package output

// Ports lib/kamal/otel_shipper.rb at Kamal 2.12.0.

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/tags"
)

const (
	batchSize     = 100
	flushInterval = 5 * time.Second
)

// otelTagKeys maps Kamal's tag names to OTel semantic convention keys. Other tags
// become "kamal.<name>".
var otelTagKeys = map[string]string{
	"service": "service.namespace", "version": "kamal.deploy_version",
	"performer": "kamal.performer", "destination": "deployment.environment.name",
}

type severityInfo struct {
	number int
	text   string
}

var severities = map[Severity]severityInfo{
	DebugSeverity: {5, "DEBUG"}, InfoSeverity: {9, "INFO"}, WarnSeverity: {13, "WARN"},
	ErrorSeverity: {17, "ERROR"}, FatalSeverity: {21, "FATAL"},
}

// ShipperDeps are the clock, the HTTP client, and the stream for shipping errors.
type ShipperDeps struct {
	Now     func() time.Time
	Poster  Poster
	Stderr  io.Writer
	Version string
}

// OtelShipper buffers OTLP log records and posts them to <endpoint>/v1/logs in
// batches of 100, every 5 seconds or when a batch fills. The first shipping
// failure goes to stderr, and later failures stay silent.
type OtelShipper struct {
	url      string
	runID    string
	resource []Attribute
	deps     ShipperDeps

	mu          sync.Mutex
	buffer      []logRecord
	errorLogged bool
	flushMu     sync.Mutex

	signal   chan struct{}
	stop     chan struct{}
	stopOnce sync.Once
	stopped  chan struct{}
	started  bool
}

// NewOtelShipper returns a shipper that has not started its flush loop yet.
//
//	shipper := output.NewOtelShipper("http://localhost:4318", deployTags, deps)
//	shipper.Start()
func NewOtelShipper(endpoint string, deployTags tags.Tags, deps ShipperDeps) *OtelShipper {
	s := &OtelShipper{url: strings.TrimSuffix(endpoint, "/") + "/v1/logs", runID: newUUID(), deps: deps,
		signal: make(chan struct{}, 1), stop: make(chan struct{}), stopped: make(chan struct{})}
	s.resource = append([]Attribute{StringAttr("service.name", "kamal"), StringAttr("service.version", deps.Version),
		StringAttr("kamal.run_id", s.runID)}, tagAttributes(deployTags)...)
	return s
}

func tagAttributes(deployTags tags.Tags) []Attribute {
	attrs := make([]Attribute, 0, len(deployTags))
	for _, tag := range deployTags {
		key, ok := otelTagKeys[tag.Name]
		if !ok {
			key = "kamal." + tag.Name
		}
		attrs = append(attrs, StringAttr(key, tag.Value))
	}
	return attrs
}

// RunID identifies this run in every record, as kamal.run_id.
func (s *OtelShipper) RunID() string { return s.runID }

// Start runs the flush loop until Shutdown.
func (s *OtelShipper) Start() {
	s.mu.Lock()
	s.started = true
	s.mu.Unlock()
	go func() {
		defer close(s.stopped)
		for {
			select {
			case <-s.stop:
				return
			case <-s.signal:
			case <-time.After(flushInterval):
			}
			s.Flush()
		}
	}()
}

// Append adds one record per line of text. Empty lines stay, and
// NoSeverity becomes INFO.
func (s *OtelShipper) Append(text, host, iostream string, severity Severity) {
	attrs := contextAttributes(host, iostream)
	for _, line := range rubyLines(text) {
		s.enqueue(s.record(line, severity, "", attrs))
	}
}

// Event adds a structured record whose body and eventName are name.
func (s *OtelShipper) Event(name string, severity Severity, attrs ...Attribute) {
	s.enqueue(s.record(name, severity, name, attrs))
}

func (s *OtelShipper) record(body string, severity Severity, event string, attrs []Attribute) logRecord {
	info, ok := severities[severity]
	if !ok {
		info = severities[InfoSeverity]
	}
	now := strconv.FormatInt(s.deps.Now().UnixNano(), 10)
	return logRecord{TimeUnixNano: now, ObservedTimeUnixNano: now, SeverityNumber: info.number, SeverityText: info.text,
		Body: stringValue(body), EventName: event, Attributes: attrs}
}

func (s *OtelShipper) enqueue(record logRecord) {
	s.mu.Lock()
	s.buffer = append(s.buffer, record)
	full := len(s.buffer) >= batchSize
	s.mu.Unlock()
	if full {
		select {
		case s.signal <- struct{}{}:
		default:
		}
	}
}

// Flush posts every buffered record.
func (s *OtelShipper) Flush() {
	s.flushMu.Lock()
	defer s.flushMu.Unlock()
	if records := s.drain(); len(records) > 0 {
		s.ship(records)
	}
}

// Shutdown stops the flush loop, waiting at most 6 seconds, and posts what is left.
// Records appended later stay in the buffer until the next Flush.
func (s *OtelShipper) Shutdown() {
	s.stopOnce.Do(func() {
		close(s.stop)
		if !s.isStarted() {
			return
		}
		select {
		case <-s.stopped:
		case <-time.After(flushInterval + time.Second):
		}
	})
	s.Flush()
}

func (s *OtelShipper) isStarted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.started
}

func (s *OtelShipper) drain() []logRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	records := s.buffer
	s.buffer = nil
	return records
}

func (s *OtelShipper) ship(records []logRecord) {
	for start := 0; start < len(records); start += batchSize {
		batch := records[start:min(start+batchSize, len(records))]
		if !s.shipBatch(batch) {
			return
		}
	}
}

// shipBatch posts one batch and reports whether the connection worked.
func (s *OtelShipper) shipBatch(batch []logRecord) bool {
	body, err := json.Marshal(s.payload(batch))
	if err != nil {
		s.logFailure(err.Error())
		return false
	}
	status, text, err := s.deps.Poster.Post(s.url, body)
	if err != nil {
		s.logFailure(err.Error())
		return false
	}
	if status < 200 || status > 299 {
		s.logFailure(fmt.Sprintf("HTTP %d %s", status, text))
	}
	return true
}

func (s *OtelShipper) payload(batch []logRecord) logsPayload {
	return logsPayload{ResourceLogs: []resourceLogs{{Resource: resource{Attributes: s.resource},
		ScopeLogs: []scopeLogs{{Scope: scope{Name: "kamal", Version: s.deps.Version}, LogRecords: batch}}}}}
}

func (s *OtelShipper) logFailure(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.errorLogged {
		s.errorLogged = true
		fmt.Fprintf(s.deps.Stderr, "OTel log shipping failed: %s\n", message)
	}
}

func contextAttributes(host, iostream string) []Attribute {
	var attrs []Attribute
	if host != "" {
		attrs = append(attrs, StringAttr("server.address", host))
	}
	if iostream != "" {
		attrs = append(attrs, StringAttr("log.iostream", iostream))
	}
	return attrs
}

// rubyLines splits text as Ruby's each_line with chomp: "a\n\nb\n" gives "a", "",
// and "b".
func rubyLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.SplitAfter(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for i, line := range lines {
		lines[i] = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	}
	return lines
}

// newUUID returns a random version 4 UUID, as SecureRandom.uuid does.
func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
