package utils

import (
	"reflect"
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

func words(t *testing.T, got interface{ String() string }) string {
	t.Helper()
	return got.String()
}

func TestArgumentize(t *testing.T) {
	attrs := []Attr{
		{"foo", yamlord.NewString("`bar`")}, {"baz", yamlord.NewString("qux")},
		{"quux", nil}, {"quuz", yamlord.NewBool(false)},
	}
	got := Argumentize("--label", attrs, false).String()
	want := "--label foo=\"\\`bar\\`\" --label baz=\"qux\" --label quux --label quuz=false"
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestArgumentizeWithRedacted(t *testing.T) {
	line := Argumentize("--label", []Attr{{"foo", yamlord.NewString("bar")}}, true)
	last := line[len(line)-1]
	if !last.IsSensitive() || last.String() != `foo="bar"` || last.Redacted() != "foo=[REDACTED]" {
		t.Fatalf("last word = %q / %q", last.String(), last.Redacted())
	}
}

func TestOptionize(t *testing.T) {
	attrs := []Attr{{"foo", yamlord.NewString("bar")}, {"baz", yamlord.NewString("qux")}, {"quux", yamlord.NewBool(true)}}
	if got := words(t, Optionize(attrs)); got != `--foo "bar" --baz "qux" --quux` {
		t.Fatalf("optionize = %s", got)
	}
}

func TestOptionizeWith(t *testing.T) {
	attrs := []Attr{{"foo", yamlord.NewString("bar")}, {"baz", yamlord.NewString("qux")}, {"quux", yamlord.NewBool(true)}}
	if got := words(t, OptionizeWith(attrs, "=")); got != `--foo="bar" --baz="qux" --quux` {
		t.Fatalf("optionize with = %s", got)
	}
}

func TestOptionizeArrayRepeatsFlag(t *testing.T) {
	attrs := []Attr{{"add-host", yamlord.Strings("a:1", "b:2")}}
	if got := words(t, Optionize(attrs)); got != `--add-host "a:1" --add-host "b:2"` {
		t.Fatalf("optionize array = %s", got)
	}
}

func TestEscapeShellValue(t *testing.T) {
	cases := map[string]string{
		"foo":                               `"foo"`,
		"`foo`":                             "\"\\`foo\\`\"",
		"${PWD}":                            `"${PWD}"`,
		"${cat /etc/hostname}":              `"${cat /etc/hostname}"`,
		"${PWD]":                            `"\${PWD]"`,
		"$(PWD)":                            `"\$(PWD)"`,
		"$PWD":                              `"\$PWD"`,
		"^(https?://)www.example.com/(.*)$": `"^(https?://)www.example.com/(.*)\$"`,
		"https://example.com/$2":            `"https://example.com/\$2"`,
	}
	for in, want := range cases {
		if got := EscapeShellValue(in); got != want {
			t.Errorf("EscapeShellValue(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestEscapeShellValueKeepsNonASCIIRuns(t *testing.T) {
	if got := EscapeShellValue("a é b"); got != `"a "é" b"` {
		t.Fatalf("got %s", got)
	}
}

// TestRubyDump checks results that Ruby 3.4 printed for String#dump.
func TestRubyDump(t *testing.T) {
	cases := map[string]string{
		`a"b\c`: `"a\"b\\c"`, "tab\tx": `"tab\tx"`, "\x1b\x01\x7f": `"\e\x01\x7F"`,
		"#{x} #$y #@z #a": `"\#{x} \#$y \#@z #a"`, "a é b": `"a \u00E9 b"`,
	}
	for in, want := range cases {
		if got := RubyDump(in); got != want {
			t.Errorf("RubyDump(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestFilterSpecificItems(t *testing.T) {
	items := []string{"web", "workers", "web-canary", ".hidden", "1.1.1.1", "1.1.1.2"}
	cases := map[string][]string{
		"web":        {"web"},
		"web*":       {"web", "web-canary"},
		"w{eb,ork}*": {"web", "workers", "web-canary"},
		"1.1.1.?":    {"1.1.1.1", "1.1.1.2"},
		"*":          {"web", "workers", "web-canary", "1.1.1.1", "1.1.1.2"},
		"[w]orkers":  {"workers"},
	}
	for filter, want := range cases {
		if got := FilterSpecificItems([]string{filter}, items); !reflect.DeepEqual(got, want) {
			t.Errorf("filter %q = %v, want %v", filter, got, want)
		}
	}
}

func TestFilterKeepsFilterOrderWithoutDuplicates(t *testing.T) {
	got := FilterSpecificItems([]string{"workers", "w*"}, []string{"web", "workers"})
	if !reflect.DeepEqual(got, []string{"workers", "web"}) {
		t.Fatalf("got %v", got)
	}
}

func TestJoinCommands(t *testing.T) {
	if got := JoinCommands([]string{" a ", "b\n"}); got != "a b" {
		t.Fatalf("got %q", got)
	}
}

func TestDockerArch(t *testing.T) {
	for out, want := range map[string]string{"x86_64\n": "amd64", "aarch64\n": "arm64", "riscv64\n": "riscv64"} {
		sh := &localshell.Fake{Results: map[string]localshell.Result{"docker info --format '{{.Architecture}}'": {Stdout: out, Success: true}}}
		if got, err := DockerArch(sh); err != nil || got != want {
			t.Errorf("DockerArch(%q) = %q, %v; want %q", out, got, err, want)
		}
	}
}

func TestOlderVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"v0.8.0", "v0.9.2", true}, {"v0.9.2", "v0.9.2", false}, {"v0.10.0", "v0.9.2", false},
		{"1.0", "1.0.0", false}, {"1.0.0.rc1", "1.0.0", true}, {"2.12.0", "2.2.0", false},
	}
	for _, c := range cases {
		if got := OlderVersion(c.a, c.b); got != c.want {
			t.Errorf("OlderVersion(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
