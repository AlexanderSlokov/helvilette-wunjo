package output

// Tests Broadcast, which stands for ActiveSupport's BroadcastLogger and the
// "modify.kamal" subscription of base_logger.rb.

import (
	"reflect"
	"testing"
	"time"
)

// recordingLogger records the calls that it receives.
type recordingLogger struct{ calls []string }

func (r *recordingLogger) Start(p Payload) error {
	r.calls = append(r.calls, "start "+p.Command)
	return nil
}
func (r *recordingLogger) Finish(_ Payload, runtime float64) {
	r.calls = append(r.calls, "finish "+rubyFloat(runtime))
}
func (r *recordingLogger) Write(e Entry) { r.calls = append(r.calls, "write "+e.Text) }
func (r *recordingLogger) Close()        { r.calls = append(r.calls, "close") }

func TestBroadcastTimesCommandsAndStopsEventsAfterClose(t *testing.T) {
	clock := testTime
	logs := NewBroadcast(func() time.Time { return clock })
	logger := &recordingLogger{}
	logs.Add(logger)
	_ = logs.Start(Payload{Command: "deploy"})
	clock = clock.Add(1260 * time.Millisecond)
	logs.Write(Entry{Text: "line\n"})
	logs.Finish(Payload{Command: "deploy"})
	logs.Close()
	_ = logs.Start(Payload{Command: "deploy"})
	logs.Write(Entry{Text: "late\n"})
	want := []string{"start deploy", "write line\n", "finish 1.3", "close", "write late\n"}
	if !reflect.DeepEqual(logger.calls, want) {
		t.Fatalf("calls = %q, want %q", logger.calls, want)
	}
}

func TestRubyLinesAndFloat(t *testing.T) {
	if got := rubyLines("a\r\n\nb"); !reflect.DeepEqual(got, []string{"a", "", "b"}) {
		t.Fatalf("rubyLines = %q", got)
	}
	if rubyFloat(12) != "12.0" || rubyFloat(0.4) != "0.4" {
		t.Fatalf("rubyFloat = %s, %s", rubyFloat(12), rubyFloat(0.4))
	}
}
