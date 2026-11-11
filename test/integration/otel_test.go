//go:build integration

package integration

// Ports the deploy log and OTel helpers of test/integration/integration_test.rb at
// Kamal 2.12.0. The collector in docker/otel_collector writes each OTLP payload to
// /tmp/otel/*.json in the deployer.

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

type otlpPayload struct {
	ResourceLogs []struct {
		Resource struct {
			Attributes []otlpAttribute `json:"attributes"`
		} `json:"resource"`
		ScopeLogs []struct {
			LogRecords []otlpRecord `json:"logRecords"`
		} `json:"scopeLogs"`
	} `json:"resourceLogs"`
}

type otlpRecord struct {
	EventName  string          `json:"eventName"`
	Body       otlpValue       `json:"body"`
	Attributes []otlpAttribute `json:"attributes"`
}

type otlpAttribute struct {
	Key   string    `json:"key"`
	Value otlpValue `json:"value"`
}

type otlpValue struct {
	StringValue *string  `json:"stringValue"`
	DoubleValue *float64 `json:"doubleValue"`
}

// attr returns the string value of key, or "" when the record has none.
func (r otlpRecord) attr(key string) string { return attrOf(r.Attributes, key) }

func attrOf(attrs []otlpAttribute, key string) string {
	for _, a := range attrs {
		if a.Key == key && a.Value.StringValue != nil {
			return *a.Value.StringValue
		}
	}
	return ""
}

var ansiCodes = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// deployLogContent returns the deploy logs that match pattern, without colors.
func (s *stack) deployLogContent(pattern string) string {
	s.t.Helper()
	return ansiCodes.ReplaceAllString(s.deployer("/", "sh", "-c", "cat /tmp/kamal-deploy-logs/"+pattern), "")
}

// assertDeployLog checks the log of one command for each pattern and the result line.
func (s *stack) assertDeployLog(pattern string, lines ...string) {
	s.t.Helper()
	content := s.deployLogContent(pattern)
	for _, line := range append(lines, `# Completed in \d+\.\d+s`) {
		if !regexp.MustCompile(line).MatchString(content) {
			s.t.Fatalf("deploy log %s lacks %s:\n%s", pattern, line, content)
		}
	}
}

func (s *stack) otelPayloads() []otlpPayload {
	s.t.Helper()
	var payloads []otlpPayload
	for _, file := range strings.Fields(s.deployer("/", "sh", "-c", "ls /tmp/otel/*.json")) {
		var payload otlpPayload
		if err := json.Unmarshal([]byte(s.deployer("/", "cat", file)), &payload); err != nil {
			s.t.Fatalf("%s is not OTLP JSON: %v", file, err)
		}
		payloads = append(payloads, payload)
	}
	return payloads
}

func (s *stack) otelRecords() []otlpRecord {
	var records []otlpRecord
	for _, payload := range s.otelPayloads() {
		for _, logs := range payload.ResourceLogs {
			for _, scope := range logs.ScopeLogs {
				records = append(records, scope.LogRecords...)
			}
		}
	}
	return records
}

func events(records []otlpRecord) []otlpRecord {
	var out []otlpRecord
	for _, r := range records {
		if r.EventName != "" {
			out = append(out, r)
		}
	}
	return out
}

// waitForOtelEvents waits up to 3 seconds for the collector to hold expected events.
func (s *stack) waitForOtelEvents(expected int) []otlpRecord {
	deadline := time.Now().Add(3 * time.Second)
	for {
		found := events(s.otelRecords())
		if len(found) >= expected || time.Now().After(deadline) {
			return found
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *stack) otelResourceAttributes() []otlpAttribute {
	payloads := s.otelPayloads()
	if len(payloads) == 0 || len(payloads[0].ResourceLogs) == 0 {
		return nil
	}
	return payloads[0].ResourceLogs[0].Resource.Attributes
}
