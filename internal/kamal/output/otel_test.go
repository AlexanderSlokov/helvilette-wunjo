package output

// Ports test/otel_shipper_test.rb and test/output/otel_logger_test.rb at Kamal
// 2.12.0. The shipper's flush loop starts only where a test calls Start, as the Ruby
// tests stub start_flush_thread.

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/tags"
)

var testTags = tags.Tags{{Name: "performer", Value: "deployer"}, {Name: "service", Value: "myapp"},
	{Name: "version", Value: "abc123"}, {Name: "destination", Value: "production"}}

func newTestShipper(poster *fakePoster, stderr *bytes.Buffer) *OtelShipper {
	return NewOtelShipper("http://localhost:4318/", testTags, ShipperDeps{Now: fixedNow, Poster: poster, Stderr: stderr, Version: "0.1.0"})
}

func TestShipperSplitsLinesAndKeepsEmptyLines(t *testing.T) {
	s := newTestShipper(&fakePoster{}, &bytes.Buffer{})
	s.Append("hello world", "", "", NoSeverity)
	s.Append("line one\nline two\n", "", "", NoSeverity)
	s.Append("before\n\nafter\n", "", "", NoSeverity)
	want := []string{"hello world", "line one", "line two", "before", "", "after"}
	if got := bodies(s.drain()); !reflect.DeepEqual(got, want) {
		t.Fatalf("bodies = %q, want %q", got, want)
	}
}

func TestShipperEventBuffersStructuredRecord(t *testing.T) {
	s := newTestShipper(&fakePoster{}, &bytes.Buffer{})
	s.Event("kamal.start", InfoSeverity, StringAttr("kamal.deploy_version", "abc123"))
	record := s.drain()[0]
	if *record.Body.StringValue != "kamal.start" || record.EventName != "kamal.start" || record.ObservedTimeUnixNano == "" {
		t.Fatalf("record = %+v", record)
	}
	attr(t, record.Attributes, "kamal.deploy_version")
}

func TestShipperAppendAddsContextAndSeverity(t *testing.T) {
	s := newTestShipper(&fakePoster{}, &bytes.Buffer{})
	s.Append("output line", "1.1.1.1", "stdout", DebugSeverity)
	s.Append("info line", "", "", InfoSeverity)
	s.Append("warn line", "", "", WarnSeverity)
	s.Append("plain line", "", "", NoSeverity)
	records := s.drain()
	if *attr(t, records[0].Attributes, "server.address").StringValue != "1.1.1.1" || *attr(t, records[0].Attributes, "log.iostream").StringValue != "stdout" {
		t.Fatalf("attributes = %+v", records[0].Attributes)
	}
	got := []int{records[0].SeverityNumber, records[1].SeverityNumber, records[2].SeverityNumber, records[3].SeverityNumber}
	if !reflect.DeepEqual(got, []int{5, 9, 13, 9}) || records[0].SeverityText != "DEBUG" || records[3].Attributes != nil {
		t.Fatalf("severities = %v, records = %+v", got, records)
	}
}

func TestShipperEventSeverityAndTypedAttributes(t *testing.T) {
	s := newTestShipper(&fakePoster{}, &bytes.Buffer{})
	s.Event("kamal.failed", ErrorSeverity, DoubleAttr("kamal.runtime", 1.5), IntAttr("retries", 3), StringsAttr("hosts", []string{"1.1.1.1", "2.2.2.2"}))
	record := s.drain()[0]
	if record.SeverityNumber != 17 || record.SeverityText != "ERROR" {
		t.Fatalf("record = %+v", record)
	}
	if *attr(t, record.Attributes, "kamal.runtime").DoubleValue != 1.5 || *attr(t, record.Attributes, "retries").IntValue != 3 {
		t.Fatalf("attributes = %+v", record.Attributes)
	}
	if hosts := attr(t, record.Attributes, "hosts").ArrayValue.Values; len(hosts) != 2 || *hosts[1].StringValue != "2.2.2.2" {
		t.Fatalf("hosts = %+v", hosts)
	}
}

func TestShipperFlushShipsWithResourceAndScope(t *testing.T) {
	poster := &fakePoster{}
	s := newTestShipper(poster, &bytes.Buffer{})
	s.Append("test log line", "", "", NoSeverity)
	s.Event("kamal.complete", InfoSeverity, StringAttr("status", "success"))
	s.Flush()
	if got := bodies(poster.records()); !reflect.DeepEqual(got, []string{"test log line", "kamal.complete"}) {
		t.Fatalf("shipped = %q", got)
	}
	logs := poster.bodies[0].ResourceLogs[0]
	want := map[string]string{"service.name": "kamal", "service.version": "0.1.0", "service.namespace": "myapp", "kamal.run_id": s.RunID(),
		"kamal.deploy_version": "abc123", "kamal.performer": "deployer", "deployment.environment.name": "production"}
	for key, value := range want {
		if got := *attr(t, logs.Resource.Attributes, key).StringValue; got != value {
			t.Errorf("resource %s = %q, want %q", key, got, value)
		}
	}
	if logs.ScopeLogs[0].Scope != (scope{Name: "kamal", Version: "0.1.0"}) {
		t.Fatalf("scope = %+v", logs.ScopeLogs[0].Scope)
	}
}

func TestShipperAcceptsLinesAfterShutdown(t *testing.T) {
	s := newTestShipper(&fakePoster{}, &bytes.Buffer{})
	s.Shutdown()
	s.Append("still works", "", "", NoSeverity)
	if got := bodies(s.drain()); !reflect.DeepEqual(got, []string{"still works"}) {
		t.Fatalf("bodies = %q", got)
	}
}

func TestShipperLogsTheFirstFailureOnly(t *testing.T) {
	stderr := &bytes.Buffer{}
	s := newTestShipper(&fakePoster{err: errors.New("connection refused")}, stderr)
	s.Append("line", "", "", NoSeverity)
	s.Flush()
	s.Append("another line", "", "", NoSeverity)
	s.Flush()
	if got := stderr.String(); got != "OTel log shipping failed: connection refused\n" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestShipperReportsHTTPErrors(t *testing.T) {
	stderr := &bytes.Buffer{}
	s := newTestShipper(&fakePoster{status: 500}, stderr)
	s.Append("line", "", "", NoSeverity)
	s.Flush()
	if got := stderr.String(); got != "OTel log shipping failed: HTTP 500 Internal Server Error\n" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestShipperFlushLoopShipsAFullBatchAndShutdownShipsTheRest(t *testing.T) {
	poster := &fakePoster{}
	s := newTestShipper(poster, &bytes.Buffer{})
	s.Start()
	s.Append(strings.Repeat("line\n", batchSize+1), "", "", NoSeverity)
	s.Shutdown()
	if got := len(poster.records()); got != batchSize+1 {
		t.Fatalf("shipped %d records, want %d", got, batchSize+1)
	}
}

func newTestOtelLogger() (*OtelLogger, *OtelShipper, *bytes.Buffer) {
	shipper, stdout := newTestShipper(&fakePoster{}, &bytes.Buffer{}), &bytes.Buffer{}
	return NewOtelLogger("http://localhost:4318", "myapp", shipper, stdout), shipper, stdout
}

func eventAttrs(t *testing.T, shipper *OtelShipper) (string, map[string]string) {
	t.Helper()
	records := shipper.drain()
	record := records[len(records)-1]
	values := map[string]string{}
	for _, a := range record.Attributes {
		if a.Value.StringValue != nil {
			values[a.Key] = *a.Value.StringValue
		}
	}
	return record.EventName, values
}

func TestOtelLoggerStartEvents(t *testing.T) {
	logger, shipper, _ := newTestOtelLogger()
	_ = logger.Start(Payload{Command: "app", Subcommand: "boot"})
	if name, attrs := eventAttrs(t, shipper); name != "kamal.start" || !reflect.DeepEqual(attrs, map[string]string{"kamal.command": "app boot"}) {
		t.Fatalf("event %s %v", name, attrs)
	}
	_ = logger.Start(Payload{Command: "deploy"})
	_, attrs := eventAttrs(t, shipper)
	if attrs["kamal.command"] != "deploy" || attrs["deployment.name"] != "deploy myapp" || attrs["deployment.id"] != shipper.RunID() {
		t.Fatalf("attrs = %v", attrs)
	}
}

func TestOtelLoggerFinishEvents(t *testing.T) {
	cases := []struct {
		payload Payload
		event   string
		attrs   map[string]string
	}{
		{Payload{Command: "deploy"}, "kamal.complete", map[string]string{"kamal.command": "deploy", "deployment.status": "succeeded", "deployment.name": "deploy myapp"}},
		{Payload{Command: "deploy", Exception: &Exception{"RuntimeError", "boom"}}, "kamal.failed",
			map[string]string{"kamal.command": "deploy", "exception.type": "RuntimeError", "exception.message": "boom", "deployment.status": "failed"}},
		{Payload{Command: "app", Subcommand: "boot"}, "kamal.complete", map[string]string{"kamal.command": "app boot"}},
	}
	for _, c := range cases {
		logger, shipper, stdout := newTestOtelLogger()
		logger.Finish(c.payload, 1.5)
		name, attrs := eventAttrs(t, shipper)
		for key, want := range c.attrs {
			if attrs[key] != want {
				t.Errorf("%s %s = %q, want %q", name, key, attrs[key], want)
			}
		}
		if name != c.event || stdout.String() != "Logs sent to http://localhost:4318\n" {
			t.Errorf("event %s, stdout %q", name, stdout.String())
		}
	}
}

func TestOtelLoggerShipsLinesWithTheirContext(t *testing.T) {
	logger, shipper, _ := newTestOtelLogger()
	logger.Write(Entry{Text: "output line\n", Host: "1.1.1.1", IOStream: "stdout", Severity: DebugSeverity})
	record := shipper.drain()[0]
	if *record.Body.StringValue != "output line" || record.SeverityNumber != 5 || *attr(t, record.Attributes, "server.address").StringValue != "1.1.1.1" {
		t.Fatalf("record = %+v", record)
	}
}
