package shell

import (
	"reflect"
	"testing"
)

// TestEscapeMatchesRuby checks results that Ruby 3.4 printed for String#shellescape.
func TestEscapeMatchesRuby(t *testing.T) {
	cases := map[string]string{
		"": "''", "abc": "abc", "a b": `a\ b`, "it's": `it\'s`, "a\nb": "a'\n'b", "é": `\é`,
		"x=1,y:2/z@w+-._": `x\=1,y:2/z@w+-._`,
	}
	for in, want := range cases {
		if got := Escape(in); got != want {
			t.Errorf("Escape(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSplitMatchesRuby checks results that Ruby 3.4 printed for String#shellsplit.
func TestSplitMatchesRuby(t *testing.T) {
	cases := map[string][]string{
		"a b":                       {"a", "b"},
		`a "b c" d`:                 {"a", "b c", "d"},
		`a 'b c'`:                   {"a", "b c"},
		`a\ b`:                      {"a b"},
		`"a\"b\\c\$d\x"`:            {`a"b\c$d\x`},
		`x\`:                        {`x\`},
		"a  b ":                     {"a", "b"},
		`k \{\"K\":\"p\)w\"\}`:      {"k", `{"K":"p)w"}`},
		`secrets fetch --from a ""`: {"secrets", "fetch", "--from", "a", ""},
	}
	for in, want := range cases {
		got, err := Split(in)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("Split(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestSplitUnmatchedQuote(t *testing.T) {
	_, err := Split(`a "b`)
	if err == nil || err.Error() != `Unmatched quote at 2: ..."` {
		t.Fatalf("error = %v", err)
	}
}

func TestLineRedaction(t *testing.T) {
	line := Line{Plain("docker"), Plain("login"), Sensitive("hunter2", DefaultRedaction)}
	if line.String() != "docker login hunter2" || line.Redacted() != "docker login [REDACTED]" {
		t.Fatalf("String %q Redacted %q", line.String(), line.Redacted())
	}
}

func TestConcatAndWords(t *testing.T) {
	if got := Concat(Words("a", "b"), Words("c")).String(); got != "a b c" {
		t.Fatalf("Concat = %q", got)
	}
}
