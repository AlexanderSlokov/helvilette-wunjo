package yamlord

import (
	"strings"
	"testing"
)

func mustParse(t *testing.T, src string) *Value {
	t.Helper()
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return doc
}

// TestScalarsFollowPsych checks the class and to_s that Ruby 3.4 printed for the same
// YAML with YAML.unsafe_load.
func TestScalarsFollowPsych(t *testing.T) {
	cases := []struct {
		yaml string
		kind Kind
		toS  string
	}{
		{"yes", Bool, "true"}, {"On", Bool, "true"}, {"off", Bool, "false"}, {`"yes"`, String, "yes"},
		{"1_000", Int, "1000"}, {"1,000", Int, "1000"}, {"3000:30", Int, "10801800"},
		{"8080:80", String, "8080:80"}, {"010", Int, "8"}, {"0x1F", Int, "31"}, {"0b101", Int, "5"},
		{"1.0", Float, "1.0"}, {"1e3", String, "1e3"}, {":quiet", Symbol, "quiet"}, {"~", Null, ""},
		{"2024-01-02", Date, "2024-01-02"}, {".inf", Float, "Infinity"}, {"y", String, "y"},
		{"1.5e-7", Float, "1.5e-07"}, {"-0", Int, "0"}, {"+12", Int, "12"}, {"12.50", Float, "12.5"},
		{"100000000000000000.0", Float, "1.0e+17"}, {"1_0.5", Float, "10.5"}, {"!!str 123", String, "123"},
		{"web-01", String, "web-01"}, {"192.168.0.1", String, "192.168.0.1"}, {"08", String, "08"},
	}
	for _, c := range cases {
		value := mustParse(t, "v: "+c.yaml).Map().At("v")
		if value.Kind() != c.kind || value.ToS() != c.toS {
			t.Errorf("%s: kind %d to_s %q, want kind %d to_s %q", c.yaml, value.Kind(), value.ToS(), c.kind, c.toS)
		}
	}
}

func TestEmptyValueIsNil(t *testing.T) {
	if value := mustParse(t, "v:\n").Map().At("v"); !value.IsNull() {
		t.Fatalf("v: parsed as %s, want nil", value.Inspect())
	}
}

func TestEmptyDocumentIsFalse(t *testing.T) {
	for _, src := range []string{"", "# only a comment\n"} {
		if doc := mustParse(t, src); !doc.IsFalse() {
			t.Errorf("Parse(%q) = %s, want false", src, doc.Inspect())
		}
	}
}

func TestKeyOrderSurvives(t *testing.T) {
	doc := mustParse(t, "env:\n  clear:\n    ZED: 1\n    ALPHA: 2\n    MIDDLE: 3\n")
	keys := doc.Dig("env", "clear").Map().Keys()
	if strings.Join(keys, ",") != "ZED,ALPHA,MIDDLE" {
		t.Fatalf("keys = %v, want ZED,ALPHA,MIDDLE", keys)
	}
}

// TestMergeKeyFollowsPsych checks the result Ruby 3.4 printed:
// {"b" => 2, "a" => 1, "c" => 3}.
func TestMergeKeyFollowsPsych(t *testing.T) {
	doc := mustParse(t, "base: &base\n  a: 1\n  b: 2\nm:\n  b: 9\n  <<: *base\n  c: 3\n")
	if got := doc.Map().At("m").Inspect(); got != `{"b" => 2, "a" => 1, "c" => 3}` {
		t.Fatalf("m = %s", got)
	}
}

func TestDuplicateKeyKeepsFirstPositionAndLastValue(t *testing.T) {
	doc := mustParse(t, "a: 1\nb: 3\na: 2\n")
	if got := doc.Inspect(); got != `{"a" => 2, "b" => 3}` {
		t.Fatalf("doc = %s", got)
	}
}

func TestBlockScalarIsString(t *testing.T) {
	if got := mustParse(t, "v: |\n  yes\n").Map().At("v"); got.Kind() != String || got.Text() != "yes\n" {
		t.Fatalf("v = %s", got.Inspect())
	}
}

func TestPresent(t *testing.T) {
	present := []*Value{NewString("x"), NewInt(0), NewBool(true), Strings("a"), NewFloat(0)}
	blank := []*Value{nil, NewNull(), NewBool(false), NewString("  "), NewList(), NewMap(nil)}
	for _, v := range present {
		if !v.Present() {
			t.Errorf("%s should be present", v.Inspect())
		}
	}
	for _, v := range blank {
		if v.Present() {
			t.Errorf("%s should be blank", v.Inspect())
		}
	}
}

func TestDeepMerge(t *testing.T) {
	a := mustParse(t, "x:\n  one: 1\n  two: 2\ny: a\n").Map()
	b := mustParse(t, "x:\n  two: 20\n  three: 3\nz: b\n").Map()
	if got := NewMap(a.DeepMerge(b)).Inspect(); got != `{"x" => {"one" => 1, "two" => 20, "three" => 3}, "y" => "a", "z" => "b"}` {
		t.Fatalf("deep merge = %s", got)
	}
}

func TestEqualIgnoresHashOrder(t *testing.T) {
	a := mustParse(t, "a: 1\nb: [x, y]\n")
	b := mustParse(t, "b: [x, y]\na: 1.0\n")
	if !Equal(a, b) {
		t.Fatal("hashes with the same pairs in another order should be equal")
	}
	if Equal(a, mustParse(t, "a: 1\nb: [y, x]\n")) {
		t.Fatal("arrays in another order should differ")
	}
}

func TestExceptAndDelete(t *testing.T) {
	m := mustParse(t, "a: 1\nb: 2\nc: 3\n").Map()
	if got := strings.Join(m.Except("b").Keys(), ","); got != "a,c" || m.Len() != 3 {
		t.Fatalf("except = %s, original len %d", got, m.Len())
	}
}

func TestDumpWithSymbolKeysRoundTrips(t *testing.T) {
	doc := mustParse(t, "roles: [web]\nversion: \"999\"\nlogging: [\"--log-opt\", 'max-size=\"10m\"']\nsshkit: {}\nssh:\n  port: 22\n  log_level: :fatal\n")
	text := DumpWithSymbolKeys(doc)
	if !strings.HasPrefix(text, "---\n:roles:\n- web\n:version: '999'\n") {
		t.Fatalf("dump =\n%s", text)
	}
	back := mustParse(t, text)
	if !Equal(back, doc) {
		t.Fatalf("round trip lost data:\n%s\n%s", text, back.Inspect())
	}
}

// The expected styles come from Psych 5 in the Ruby oracle image, where
// {"k" => s}.to_yaml prints each string.
func TestQuoteStringFollowsPsych(t *testing.T) {
	cases := map[string]string{
		"--log-opt": `"--log-opt"`, `max-size="10m"`: `max-size="10m"`, "999": "'999'", "dhh/app": "dhh/app",
		"y": `"y"`, "<<": "!!str '<<'", "": "''", ":fatal": `":fatal"`, "09": "'09'", "a: b": "'a: b'", "true": "'true'",
	}
	for text, want := range cases {
		if got := quoteString(text); got != want {
			t.Errorf("quoteString(%q) = %s, want %s", text, got, want)
		}
	}
}

func TestDumpWritesSymbolKeyedMapsWithColons(t *testing.T) {
	ssh := OrderedOf(Pair{"user", NewString("root")}).SymbolKeyed()
	doc := NewMap(OrderedOf(Pair{"ssh_options", NewMap(ssh)}, Pair{"builder", NewMap(OrderedOf(Pair{"arch", NewString("amd64")}))}))
	want := "---\n:ssh_options:\n  :user: root\n:builder:\n  arch: amd64\n"
	if got := DumpWithSymbolKeys(doc); got != want {
		t.Fatalf("dump = %q, want %q", got, want)
	}
	if !ssh.Clone().HasSymbolKeys() {
		t.Fatal("Clone dropped the Symbol keys")
	}
}
