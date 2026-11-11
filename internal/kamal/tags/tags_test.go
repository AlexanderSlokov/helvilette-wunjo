package tags

import (
	"testing"
	"time"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
)

var fixedTime = time.Date(2026, 11, 11, 10, 30, 0, 0, time.UTC)

func TestDefaultTagsAndString(t *testing.T) {
	d := Deploy{Destination: "staging", Version: "999", Service: "app", AbbreviatedVersion: "999"}
	got := Default(d, fixedTime, "deployer@example.com").String()
	want := "[2026-11-11T10:30:00Z] [deployer@example.com] [staging] [999] [app@999] [app]"
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestDefaultTagsWithoutDestination(t *testing.T) {
	got := Default(Deploy{Version: "999", Service: "app"}, fixedTime, "me").Except("recorded_at").String()
	if got != "[me] [999] [app] [app]" {
		t.Fatalf("got %s", got)
	}
}

func TestEnvAndWith(t *testing.T) {
	env := Tags{{"version", "1"}}.With(Tag{"hosts", "a,b"}).Env()
	if len(env) != 2 || env[0] != (Tag{"KAMAL_VERSION", "1"}) || env[1] != (Tag{"KAMAL_HOSTS", "a,b"}) {
		t.Fatalf("env = %v", env)
	}
}

func TestPerformerFallsBackToWhoami(t *testing.T) {
	sh := &localshell.Fake{Results: map[string]localshell.Result{
		"git config user.email": {Stdout: "\n", Success: false},
		"whoami":                {Stdout: "deployer\n", Success: true},
	}}
	if got := Performer(sh); got != "deployer" {
		t.Fatalf("got %q", got)
	}
}
