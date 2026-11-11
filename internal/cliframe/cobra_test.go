package cliframe

import (
	"bytes"
	"strings"
	"testing"
)

// recordingRun stores the invocation it receives, so a test can inspect the parsed
// flags and arguments.
type recordingRun struct{ got *Invocation }

func (r *recordingRun) run(inv *Invocation) error {
	r.got = inv
	return nil
}

func treeWith(rec *recordingRun) *Command {
	deploy := &Command{
		Name:  "deploy",
		Short: "Deploy app to servers",
		Flags: []Flag{{Name: "skip-push", Short: "P", Kind: BoolFlag}},
		Run:   rec.run,
	}
	logs := &Command{Name: "logs", MaxArgs: 1, Flags: []Flag{{Name: "lines", Short: "n", Kind: IntFlag, Default: "100"}}, Run: rec.run}
	return &Command{
		Name:        "wunjo",
		GlobalFlags: []Flag{{Name: "hosts", Short: "h", Kind: StringFlag}, {Name: "verbose", Short: "v", Kind: BoolFlag}},
		Children:    []*Command{deploy, logs},
	}
}

func execute(t *testing.T, rec *recordingRun, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := Execute(treeWith(rec), args, &out, &out)
	return out.String(), err
}

func TestShortHFlagMeansHosts(t *testing.T) {
	rec := &recordingRun{}
	if _, err := execute(t, rec, "deploy", "-h", "web-1", "-P"); err != nil {
		t.Fatal(err)
	}
	if rec.got.String("hosts") != "web-1" || !rec.got.Bool("skip-push") {
		t.Fatalf("hosts=%q skip-push=%v, want web-1 and true", rec.got.String("hosts"), rec.got.Bool("skip-push"))
	}
}

func TestUnderscoreFlagNamesWork(t *testing.T) {
	rec := &recordingRun{}
	if _, err := execute(t, rec, "deploy", "--skip_push"); err != nil {
		t.Fatal(err)
	}
	if !rec.got.Bool("skip-push") {
		t.Fatal("--skip_push did not set skip-push")
	}
}

func TestIntFlagDefaultAndChanged(t *testing.T) {
	rec := &recordingRun{}
	if _, err := execute(t, rec, "logs"); err != nil {
		t.Fatal(err)
	}
	if rec.got.Int("lines") != 100 || rec.got.Changed("lines") {
		t.Fatalf("lines=%d changed=%v, want 100 and false", rec.got.Int("lines"), rec.got.Changed("lines"))
	}
}

func TestStringListFlagRepeats(t *testing.T) {
	rec := &recordingRun{}
	tree := &Command{Name: "wunjo", Children: []*Command{{Name: "exec", Flags: []Flag{{Name: "env", Short: "e", Kind: StringListFlag}}, Run: rec.run}}}
	var out bytes.Buffer
	if err := Execute(tree, []string{"exec", "-e", "A:1", "--env", "B:2"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if got := rec.got.Strings("env"); len(got) != 2 || got[0] != "A:1" || got[1] != "B:2" {
		t.Fatalf("env = %v", got)
	}
}

func TestPassUnknownFlagsKeepsThemAsArgs(t *testing.T) {
	rec := &recordingRun{}
	exec := &Command{Name: "exec", MaxArgs: AnyArgs, PassUnknownFlags: true, Flags: []Flag{{Name: "raw", Kind: BoolFlag}}, Run: rec.run}
	tree := &Command{Name: "wunjo", GlobalFlags: []Flag{{Name: "config-file", Short: "c", Kind: StringFlag}}, Children: []*Command{exec}}
	var out bytes.Buffer
	if err := Execute(tree, []string{"exec", "date", "-j", "--raw", "-c", "x.yml"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Join(rec.got.Args, " ") != "date -j" || !rec.got.Bool("raw") || rec.got.String("config-file") != "x.yml" {
		t.Fatalf("args %v raw %v config %q", rec.got.Args, rec.got.Bool("raw"), rec.got.String("config-file"))
	}
}

func TestCommandWithoutArgsRejectsArgs(t *testing.T) {
	_, err := execute(t, &recordingRun{}, "deploy", "extra")
	if err == nil {
		t.Fatal("deploy accepted a positional argument")
	}
}

func TestHelpListsChildren(t *testing.T) {
	out, err := execute(t, &recordingRun{}, "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "deploy") || !strings.Contains(out, "Deploy app to servers") {
		t.Fatalf("help output lacks the deploy command:\n%s", out)
	}
}
