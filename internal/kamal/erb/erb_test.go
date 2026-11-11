package erb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/testsupport"
)

func renderWith(t *testing.T, template string, env testsupport.FakeEnv, trim bool) (string, error) {
	t.Helper()
	return Render(template, Options{File: "config/deploy.yml", TrimMode: trim, Lookup: env.Lookup})
}

func mustRender(t *testing.T, template string, env testsupport.FakeEnv) string {
	t.Helper()
	out, err := renderWith(t, template, env, true)
	if err != nil {
		t.Fatalf("Render(%q): %v", template, err)
	}
	return out
}

func TestSupportedForms(t *testing.T) {
	env := testsupport.FakeEnv{"GPU_HOST": "gpu-node-01"}
	cases := map[string]string{
		`<%= ENV["GPU_HOST"] %>`:                      "gpu-node-01",
		`<%= ENV['GPU_HOST'] %>`:                      "gpu-node-01",
		`<%= ENV["UNSET"] %>`:                         "",
		`<%= ENV.fetch("GPU_HOST") %>`:                "gpu-node-01",
		`<%= ENV.fetch("UNSET", "cpu-node-01") %>`:    "cpu-node-01",
		`<%= ENV.fetch('GPU_HOST', 'cpu-node-01') %>`: "gpu-node-01",
		`<%= "my-user" %>`:                            "my-user",
		`<%= 'my-user' %>`:                            "my-user",
		`<%# a comment %>`:                            "",
		`<%%= literal %>`:                             "<%= literal %>",
		`<%= "a\"b\\c" %>`:                            `a"b\c`,
		`<%= 'a\'b\c' %>`:                             `a'b\c`,
		`host: <%=ENV[ "GPU_HOST" ]%>`:                "host: gpu-node-01",
	}
	for template, want := range cases {
		if got := mustRender(t, template, env); got != want {
			t.Errorf("Render(%q) = %q, want %q", template, got, want)
		}
	}
}

// TestTrimMarkers checks results that Ruby 3.4 produced for the same templates with
// ERB.new(template, trim_mode: "-").
func TestTrimMarkers(t *testing.T) {
	env := testsupport.FakeEnv{"ROLE": "web"}
	cases := map[string]string{
		"role: <%= ENV[\"ROLE\"] -%>\nnext: 1\n": "role: webnext: 1\n",
		"<%# dropped -%>\nkept\n":                "kept\n",
		"a\r\n  <%- %>x":                         "a\r\nx",
		"  <%- # note -%>\nk\n":                  "k\n",
		"a <%- %>b":                              "a b",
		"  <% %>\nz":                             "  \nz",
		"<%= \"x\" -%>\r\nb":                     "xb",
		"<%= \"q\" %>  -%>":                      "q  -%>",
	}
	for template, want := range cases {
		got, err := renderWith(t, template, env, true)
		if err != nil || got != want {
			t.Errorf("Render(%q) = %q, %v; want %q", template, got, err, want)
		}
	}
}

// TestTrimStatementWithCodeIsUnsupported covers <%-= and <%- if, which Ruby parses
// as statement tags.
func TestTrimStatementWithCodeIsUnsupported(t *testing.T) {
	for _, template := range []string{"x:\n  <%-= ENV[\"ROLE\"] %>\n", "  <%- if true -%>\n"} {
		_, err := renderWith(t, template, nil, true)
		if err == nil || !strings.Contains(err.Error(), "unsupported ERB expression") {
			t.Errorf("Render(%q) = %v, want an unsupported-expression error", template, err)
		}
	}
}

func TestTrimMarkersNeedTrimMode(t *testing.T) {
	_, err := renderWith(t, `<%= "x" -%>`, nil, false)
	if err == nil || !strings.Contains(err.Error(), `unsupported ERB expression: "x" -`) {
		t.Fatalf("Render without trim mode = %v, want an unsupported-expression error", err)
	}
}

func TestFetchWithoutDefaultFailsWhenUnset(t *testing.T) {
	_, err := renderWith(t, "a: 1\nb: <%= ENV.fetch(\"DB_HOST\") %>\n", nil, true)
	want := `config/deploy.yml:2: ENV.fetch("DB_HOST"): environment variable DB_HOST is not set`
	if err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Fatalf("error = %v, want prefix %q", err, want)
	}
}

func TestUnsupportedExpressionNamesFileLineAndExpression(t *testing.T) {
	template := strings.Repeat("\n", 10) + "  arch: <%= ArchHelper.remote_arch %>\n"
	_, err := renderWith(t, template, nil, true)
	want := "config/deploy.yml:11: unsupported ERB expression: ArchHelper.remote_arch\n" +
		"  Supported: ENV[\"NAME\"], ENV.fetch(\"NAME\"), ENV.fetch(\"NAME\", \"default\"), \"text\"\n" +
		"  Move the value into an environment variable and read it with ENV[\"NAME\"]."
	if err == nil || err.Error() != want {
		t.Fatalf("error =\n%v\nwant\n%s", err, want)
	}
}

func TestUnsupportedForms(t *testing.T) {
	templates := []string{
		`<% if true %>`,
		`<%= ENV.fetch("X") { "d" } %>`,
		`<%= "#{1 + 1}" %>`,
		`<%= %x(aws ecr get-login-password) %>`,
		`<%= ENV["A"] || "b" %>`,
		`<%= "\u00e9" %>`,
		`<%= ENV["A" %>`,
	}
	for _, template := range templates {
		_, err := renderWith(t, template, nil, true)
		if err == nil || !strings.Contains(err.Error(), "unsupported ERB expression") {
			t.Errorf("Render(%q) = %v, want an unsupported-expression error", template, err)
		}
	}
}

func TestUnterminatedTag(t *testing.T) {
	_, err := renderWith(t, "a: <%= \"x\"\n", nil, true)
	if err == nil || err.Error() != "config/deploy.yml:1: unterminated ERB tag: want a closing %>" {
		t.Fatalf("error = %v", err)
	}
}

// TestAllFixturesRender renders every Kamal fixture with trim mode, after
// testsupport.ReadFixture replaces the ArchHelper.remote_arch calls in
// deploy_with_cloud_builder.yml, deploy_with_remote_builder.yml,
// deploy_with_remote_builder_and_custom_ports.yml,
// deploy_with_local_registry_and_remote_builder.yml, and
// deploy_with_local_registry_and_remote_builder_with_port.yml.
func TestAllFixturesRender(t *testing.T) {
	env := testsupport.FakeEnv{testsupport.RemoteArchVariable: testsupport.RemoteArch}
	names, _ := filepath.Glob(filepath.Join(testsupport.FixturesDir(), "*.yml"))
	if len(names) != 44 {
		t.Fatalf("found %d deploy fixtures, want 44", len(names))
	}
	for _, path := range names {
		template, err := testsupport.ReadFixture(filepath.Base(path))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := renderWith(t, template, env, true); err != nil {
			t.Errorf("%s: %v", filepath.Base(path), err)
		}
	}
}

// TestAccessoryFileRenders ports the expectation behind test/fixtures/files/structure.sql.erb,
// which Kamal renders without trim mode and with the accessory's environment.
func TestAccessoryFileRenders(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(testsupport.FixturesDir(), "files", "structure.sql.erb"))
	if err != nil {
		t.Fatal(err)
	}
	env := testsupport.FakeEnv{"MYSQL_ROOT_HOST": "%", "MYSQL_ROOT_PASSWORD": "secret123", "ENV_VAR": "x"}
	got, err := Render(string(content), Options{File: "structure.sql.erb", Lookup: env.Lookup})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "This was dynamically expanded\n%\nsecret123\nx\n") {
		t.Fatalf("rendered %q", got)
	}
}
