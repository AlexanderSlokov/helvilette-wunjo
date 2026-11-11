package commands

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/git"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/secrets"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/testsupport"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// recordedAt is the fixed time of every test, so tags are repeatable.
var recordedAt = time.Date(2026, 11, 11, 10, 30, 0, 0, time.UTC)

// commandCase holds a deploy file and the fakes of one command test, as the
// @config, @destination, and secrets of a Kamal command test.
type commandCase struct {
	raw         *yamlord.Ordered
	destination string
	version     string
	tty         bool
	files       testsupport.FakeFiles
	processEnv  testsupport.FakeEnv
	results     map[string]localshell.Result
}

func newCase(t *testing.T, deploy string) *commandCase {
	t.Helper()
	return &commandCase{
		raw:        parseYAML(t, deploy),
		version:    "999",
		files:      testsupport.FakeFiles{},
		processEnv: testsupport.FakeEnv{},
		results: map[string]localshell.Result{
			"docker info --format '{{.Architecture}}'": {Stdout: "x86_64\n", Success: true},
			"git config user.email":                    {Stdout: "deployer@example.com\n", Success: true},
		},
	}
}

func parseYAML(t *testing.T, src string) *yamlord.Ordered {
	t.Helper()
	doc, err := yamlord.Parse([]byte(src))
	if err != nil || doc.Map() == nil {
		t.Fatalf("test YAML does not parse to a mapping: %v\n%s", err, src)
	}
	return doc.Map()
}

func parseValue(t *testing.T, src string) *yamlord.Value {
	t.Helper()
	return parseYAML(t, "v: "+src).At("v")
}

// set assigns the JSON value at path, as @config[:a]["b"] = value does.
func (c *commandCase) set(t *testing.T, value string, path ...string) {
	t.Helper()
	m := c.raw
	for _, key := range path[:len(path)-1] {
		m = m.At(key).Map()
		if m == nil {
			t.Fatalf("no mapping at %v", path)
		}
	}
	m.Set(path[len(path)-1], parseValue(t, value))
}

// extra returns a copy with a top-level key set, as new_command(**additional_config).
func (c *commandCase) extra(t *testing.T, key, value string) *commandCase {
	t.Helper()
	copy := *c
	copy.raw = c.raw.Clone()
	copy.raw.Set(key, parseValue(t, value))
	return &copy
}

func (c *commandCase) secrets(content string) *commandCase {
	c.files[".kamal/secrets"] = content
	return c
}

func (c *commandCase) shell() *localshell.Fake { return &localshell.Fake{Results: c.results} }

func (c *commandCase) config(t *testing.T) *configuration.Configuration {
	t.Helper()
	sh := c.shell()
	env := configuration.Environment{
		Lookup: c.processEnv.Lookup, Shell: sh, Git: git.Git{Shell: sh}, Files: c.files, SecretFiles: c.files,
		Substitution: secrets.Substitution{Shell: sh, Lookup: c.processEnv.Lookup},
		WorkDir:      "/home/app/project", TempDir: "/tmp", RandomHex: func() string { return "0123456789abcdef" },
	}
	cfg, err := configuration.New(c.raw, configuration.Options{Destination: c.destination, Version: c.version, Env: env})
	if err != nil {
		t.Fatalf("configuration rejected: %v", err)
	}
	return cfg
}

func (c *commandCase) runtime() Runtime {
	sh := c.shell()
	return Runtime{
		Now: func() time.Time { return recordedAt }, Performer: "deployer@example.com", Git: git.Git{Shell: sh},
		RandomHex: func(n int) string { return strings.Repeat("a1", n) }, StdinTTY: c.tty, Files: c.files, WorkDir: "/home/app/project",
	}
}

func (c *commandCase) base(t *testing.T) Base { return NewBase(c.config(t), c.runtime()) }

// env parses the env Hash of an exec command.
func (c *commandCase) env(t *testing.T, value string) *yamlord.Ordered {
	t.Helper()
	return parseValue(t, value).Map()
}

func intValue(n int64) *yamlord.Value { return yamlord.NewInt(n) }

func assertText(t *testing.T, want, got string) {
	t.Helper()
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func assertWords(t *testing.T, want []string, got Line) {
	t.Helper()
	assertText(t, strings.Join(want, "\x1f"), strings.Join(wordsOf(got), "\x1f"))
}

func assertMatch(t *testing.T, pattern, got string) {
	t.Helper()
	if !regexp.MustCompile(pattern).MatchString(got) {
		t.Fatalf("%q does not match %s", got, pattern)
	}
}

func assertNoMatch(t *testing.T, pattern, got string) {
	t.Helper()
	if regexp.MustCompile(pattern).MatchString(got) {
		t.Fatalf("%q matches %s", got, pattern)
	}
}

const appDeploy = `
service: app
image: dhh/app
registry: {username: dhh, password: secret}
servers: {web: [1.1.1.1], workers: [1.1.1.2]}
env: {secret: [RAILS_MASTER_KEY]}
builder: {arch: amd64}
`

func newAppCase(t *testing.T) *commandCase {
	return newCase(t, appDeploy).secrets("RAILS_MASTER_KEY=456")
}

func (c *commandCase) app(t *testing.T, role, host string) *App {
	t.Helper()
	cfg := c.config(t)
	return NewApp(NewBase(cfg, c.runtime()), cfg.Role(role), host)
}
