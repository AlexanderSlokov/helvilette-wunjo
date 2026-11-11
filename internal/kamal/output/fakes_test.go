package output

// Named fakes of the output tests: the file system and the OTLP endpoint.

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"sync"
	"testing"
	"time"
)

// fakeFiles keeps written files in memory, keyed by path.
type fakeFiles struct {
	mu    sync.Mutex
	dirs  []string
	files map[string]*fakeFile
}

type fakeFile struct {
	bytes.Buffer
	closed bool
}

func (f *fakeFile) Close() error {
	f.closed = true
	return nil
}

func newFakeFiles() *fakeFiles { return &fakeFiles{files: map[string]*fakeFile{}} }

func (f *fakeFiles) MkdirAll(path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dirs = append(f.dirs, path)
	return nil
}

func (f *fakeFiles) OpenAppend(path string) (io.WriteCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.files[path] == nil {
		f.files[path] = &fakeFile{}
	}
	return f.files[path], nil
}

// paths returns the file paths in order.
func (f *fakeFiles) paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var paths []string
	for path := range f.files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// fakePoster records each posted body and answers with status, or fails with err.
type fakePoster struct {
	mu     sync.Mutex
	bodies []logsPayload
	status int
	err    error
}

func (p *fakePoster) Post(_ string, body []byte) (int, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return 0, "", p.err
	}
	var payload logsPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, "", errors.New("fakePoster: body is not an OTLP payload: " + err.Error())
	}
	p.bodies = append(p.bodies, payload)
	if p.status == 0 {
		return 200, "OK", nil
	}
	return p.status, "Internal Server Error", nil
}

func (p *fakePoster) records() []logRecord {
	p.mu.Lock()
	defer p.mu.Unlock()
	var records []logRecord
	for _, body := range p.bodies {
		records = append(records, body.ResourceLogs[0].ScopeLogs[0].LogRecords...)
	}
	return records
}

var testTime = time.Date(2026, 11, 11, 10, 30, 0, 0, time.UTC)

func fixedNow() time.Time { return testTime }

func bodies(records []logRecord) []string {
	out := make([]string, len(records))
	for i, r := range records {
		out[i] = *r.Body.StringValue
	}
	return out
}

func attr(t *testing.T, attrs []Attribute, key string) Value {
	t.Helper()
	for _, a := range attrs {
		if a.Key == key {
			return a.Value
		}
	}
	t.Fatalf("no attribute %q in %+v", key, attrs)
	return Value{}
}
