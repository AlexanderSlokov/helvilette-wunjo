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

func TestThorStyleBooleanAndGreedyFlags(t *testing.T) {
	rec := &recordingRun{}
	set := &Command{Name: "set", MaxArgs: 1, Flags: []Flag{{Name: "publish", Kind: BoolFlag, Default: "true"},
		{Name: "docker-options", Kind: StringListFlag, Greedy: true}}, Run: rec.run}
	var out bytes.Buffer
	if err := Execute(&Command{Name: "wunjo", Children: []*Command{set}}, []string{"set", "x", "--publish", "false", "--docker_options", "a=1", "b=2"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if rec.got.Bool("publish") || strings.Join(rec.got.Strings("docker-options"), ",") != "a=1,b=2" || strings.Join(rec.got.Args, " ") != "x" {
		t.Fatalf("publish %v options %v args %v", rec.got.Bool("publish"), rec.got.Strings("docker-options"), rec.got.Args)
	}
}

// Thor 1.4.0 reads `--opts=a b c` as ["a", "b", "c"]. The integration suite's
// `proxy boot_config set --docker-options=label=... label=...` found the difference.
func TestGreedyFlagWithAnEqualsSignTakesTheFollowingWords(t *testing.T) {
	rec := &recordingRun{}
	set := &Command{Name: "set", MaxArgs: 1, Flags: []Flag{{Name: "docker-options", Kind: StringListFlag, Greedy: true}}, Run: rec.run}
	var out bytes.Buffer
	if err := Execute(&Command{Name: "wunjo", Children: []*Command{set}}, []string{"set", "--docker-options=a=1", "b=2", "c=3"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(rec.got.Strings("docker-options"), ","); got != "a=1,b=2,c=3" {
		t.Fatalf("options = %s", got)
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

// The expectations come from Thor 1.4.0: P.start(%w[x -e A:1 B:2 ruby -v]) gives
// {"A" => "1", "B" => "2"} and ["ruby", "-v"], and a second -e replaces the first.
func TestThorStylePairsFlags(t *testing.T) {
	cases := map[string]struct{ env, args string }{
		"-e A:1 B:2 ruby -v":    {"A:1,B:2", "ruby -v"},
		"-e A:1 -e B:2 ruby":    {"B:2", "ruby"},
		"--env=A:1 B:x:y ls":    {"A:1,B:x:y", "ls"},
		"ruby -e URL:https://x": {"URL:https://x", "ruby"},
	}
	for line, want := range cases {
		rec := &recordingRun{}
		run := &Command{Name: "exec", MaxArgs: AnyArgs, PassUnknownFlags: true, Flags: []Flag{{Name: "env", Short: "e", Kind: StringListFlag, Pairs: true}}, Run: rec.run}
		var out bytes.Buffer
		if err := Execute(&Command{Name: "wunjo", Children: []*Command{run}}, append([]string{"exec"}, strings.Fields(line)...), &out, &out); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
		if got := strings.Join(rec.got.Strings("env"), ","); got != want.env || strings.Join(rec.got.Args, " ") != want.args {
			t.Errorf("%s: env %q args %q, want %q and %q", line, got, rec.got.Args, want.env, want.args)
		}
	}
}

func TestPairsFlagRejectsARepeatedKey(t *testing.T) {
	run := &Command{Name: "exec", MaxArgs: AnyArgs, PassUnknownFlags: true, Flags: []Flag{{Name: "env", Short: "e", Kind: StringListFlag, Pairs: true}}, Run: (&recordingRun{}).run}
	var out bytes.Buffer
	err := Execute(&Command{Name: "wunjo", Children: []*Command{run}}, []string{"exec", "-e", "A:1", "A:2", "ls"}, &out, &out)
	want := "You can't specify 'A' more than once in option '--env'; got A:1 and A:2"
	if parse, ok := err.(*ParseError); !ok || parse.Message != want {
		t.Fatalf("err = %v, want ParseError %q", err, want)
	}
}
