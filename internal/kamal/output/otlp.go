package output

// The OTLP/HTTP JSON shapes that lib/kamal/otel_shipper.rb writes at Kamal 2.12.0.

import (
	"bytes"
	"fmt"
	"net/http"
	"time"
)

// Attribute is one OTLP key and typed value.
type Attribute struct {
	Key   string `json:"key"`
	Value Value  `json:"value"`
}

// Value is an OTLP AnyValue. Exactly one field is set.
type Value struct {
	StringValue *string     `json:"stringValue,omitempty"`
	IntValue    *int64      `json:"intValue,omitempty"`
	DoubleValue *float64    `json:"doubleValue,omitempty"`
	ArrayValue  *ArrayValue `json:"arrayValue,omitempty"`
}

// ArrayValue is an OTLP array of values.
type ArrayValue struct {
	Values []Value `json:"values"`
}

// StringAttr returns a string attribute, such as {"kamal.command", "deploy"}.
func StringAttr(key, value string) Attribute { return Attribute{Key: key, Value: stringValue(value)} }

// IntAttr returns an integer attribute.
func IntAttr(key string, value int64) Attribute {
	return Attribute{Key: key, Value: Value{IntValue: &value}}
}

// DoubleAttr returns a floating-point attribute, such as the runtime in seconds.
func DoubleAttr(key string, value float64) Attribute {
	return Attribute{Key: key, Value: Value{DoubleValue: &value}}
}

// StringsAttr returns an array attribute of strings, such as the hosts.
func StringsAttr(key string, values []string) Attribute {
	items := make([]Value, len(values))
	for i, v := range values {
		items[i] = stringValue(v)
	}
	return Attribute{Key: key, Value: Value{ArrayValue: &ArrayValue{Values: items}}}
}

func stringValue(text string) Value { return Value{StringValue: &text} }

// logRecord is one OTLP LogRecord. The times are nanoseconds as strings.
type logRecord struct {
	TimeUnixNano         string      `json:"timeUnixNano"`
	ObservedTimeUnixNano string      `json:"observedTimeUnixNano"`
	SeverityNumber       int         `json:"severityNumber"`
	SeverityText         string      `json:"severityText"`
	Body                 Value       `json:"body"`
	EventName            string      `json:"eventName,omitempty"`
	Attributes           []Attribute `json:"attributes,omitempty"`
}

type logsPayload struct {
	ResourceLogs []resourceLogs `json:"resourceLogs"`
}

type resourceLogs struct {
	Resource  resource    `json:"resource"`
	ScopeLogs []scopeLogs `json:"scopeLogs"`
}

type resource struct {
	Attributes []Attribute `json:"attributes"`
}

type scopeLogs struct {
	Scope      scope       `json:"scope"`
	LogRecords []logRecord `json:"logRecords"`
}

type scope struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Poster sends one JSON body to an OTLP endpoint and returns the HTTP status.
type Poster interface {
	Post(url string, body []byte) (status int, text string, err error)
}

// HTTPPoster posts with net/http, with Kamal's timeouts: 2 seconds to connect and 5
// seconds to read.
type HTTPPoster struct{ Client *http.Client }

// NewHTTPPoster returns a poster with Kamal's timeouts.
func NewHTTPPoster() HTTPPoster {
	return HTTPPoster{Client: &http.Client{Timeout: 7 * time.Second, Transport: &http.Transport{
		ResponseHeaderTimeout: 5 * time.Second, TLSHandshakeTimeout: 2 * time.Second,
	}}}
}

// Post sends body as application/json.
func (p HTTPPoster) Post(url string, body []byte) (int, string, error) {
	response, err := p.Client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, "", fmt.Errorf("POST %s: %w", url, err)
	}
	defer response.Body.Close()
	return response.StatusCode, http.StatusText(response.StatusCode), nil
}
