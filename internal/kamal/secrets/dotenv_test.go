package secrets

import (
	"strings"
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
)

// dotenvCases is the input that Ruby 3.4 with dotenv 3.2.0 parsed into
// dotenvExpected. Only the command runner is faked.
const dotenvCases = `A=plain
B = spaced value   # comment
C='single $A \n quoted'
D="double $A \n quoted"
E="multi
line"
export F=exported
G=${A}x$A}$
H=\$A escaped
I=a\b\c\\d
J="esc \"q\" x"
K: colon
L=
M=#notcomment?
N=value#hash
O=$(echo hi)
P=\$(echo no)
Q=$(echo "a(b)c")
R=${UNSET_VAR}end
S="tab\tx \r y"
T=$(printf '%s' '$A')
U=${A
V=a.b.c
W.X=dotkey
Y="unterminated
Z=last
`

var dotenvExpected = [][2]string{
	{"A", "plain"}, {"B", "spaced value"}, {"C", `single $A \n quoted`}, {"D", `double plain \n quoted`},
	{"E", "multi\nline"}, {"F", "exported"}, {"G", "plainxplain$"}, {"H", "$A escaped"}, {"I", `abc\d`},
	{"J", `esc "q" x`}, {"K", "colon"}, {"L", ""}, {"M", ""}, {"N", "value"}, {"O", "hi"},
	{"P", "$(echo no)"}, {"Q", "a(b)c"}, {"R", "end"}, {"S", `tabtx \r y`}, {"T", "plain"}, {"U", "plain"},
	{"V", "a.b.c"}, {"W.X", "dotkey"}, {"Y", `"unterminated`}, {"Z", "last"},
}

func TestParseDotenvMatchesRuby(t *testing.T) {
	sh := &localshell.Fake{Results: map[string]localshell.Result{
		"echo hi":             {Stdout: "hi\n", Success: true},
		`echo "a(b)c"`:        {Stdout: "a(b)c\n", Success: true},
		`printf '%s' 'plain'`: {Stdout: "plain", Success: true},
	}}
	values, err := ParseDotenv(dotenvCases, Substitution{Shell: sh})
	if err != nil {
		t.Fatal(err)
	}
	keys := values.Keys()
	if len(keys) != len(dotenvExpected) {
		t.Fatalf("keys = %v", keys)
	}
	for i, want := range dotenvExpected {
		if keys[i] != want[0] || values.At(want[0]).ToS() != want[1] {
			t.Errorf("%d: %s=%q, want %s=%q", i, keys[i], values.At(keys[i]).ToS(), want[0], want[1])
		}
	}
}

func TestExportWithoutValueOfUnsetVariableFails(t *testing.T) {
	_, err := ParseDotenv("export MISSING\n", Substitution{})
	if err == nil || !strings.Contains(err.Error(), "has an unset variable") {
		t.Fatalf("error = %v", err)
	}
}

// TestInlinesKamalSecretsCommands ports test/secrets/dotenv_inline_command_substitution_test.rb.
func TestInlinesKamalSecretsCommands(t *testing.T) {
	var got []string
	inline := func(args []string) (string, error) {
		got = args
		return "results", nil
	}
	out, err := InlineCommandSubstitution("FOO=$(kamal secrets fetch ...)", Substitution{InlineSecrets: inline})
	if err != nil || out != "FOO=results" {
		t.Fatalf("got %q, %v", out, err)
	}
	if strings.Join(got, " ") != "secrets fetch ... --inline" {
		t.Fatalf("inline args = %q", got)
	}
}

func TestExecutesOtherCommands(t *testing.T) {
	sh := &localshell.Fake{Results: map[string]localshell.Result{"blah": {Stdout: "results", Success: true}}}
	out, err := InlineCommandSubstitution("FOO=$(blah)", Substitution{Shell: sh})
	if err != nil || out != "FOO=results" {
		t.Fatalf("got %q, %v", out, err)
	}
}

func TestHandlesEscapedParenthesesInCommandArguments(t *testing.T) {
	var got []string
	inline := func(args []string) (string, error) {
		got = args
		return "pass)word", nil
	}
	line := `KEY1=$(kamal secrets extract KEY1 \{\"KEY1\":\"pass\)word\"\})`
	out, err := InlineCommandSubstitution(line, Substitution{InlineSecrets: inline})
	if err != nil || out != "KEY1=pass)word" {
		t.Fatalf("got %q, %v", out, err)
	}
	if len(got) < 4 || strings.Join(got[:3], " ") != "secrets extract KEY1" || got[3] != `{"KEY1":"pass)word"}` {
		t.Fatalf("inline args = %q", got)
	}
}
