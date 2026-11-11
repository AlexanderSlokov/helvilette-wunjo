package configuration

import (
	"strings"
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/git"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/secrets"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/testsupport"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// baseDeploy is the @deploy hash that most Kamal configuration tests start from.
const baseDeploy = `
service: app
image: dhh/app
registry:
  username: dhh
  password: secret
builder:
  arch: amd64
env:
  REDIS_URL: redis://x/y
servers:
  - 1.1.1.1
  - 1.1.1.2
volumes:
  - /local/path:/container/path
`

// parseYAML parses a YAML mapping for a test.
func parseYAML(t *testing.T, src string) *yamlord.Ordered {
	t.Helper()
	doc, err := yamlord.Parse([]byte(src))
	if err != nil || doc.Map() == nil {
		t.Fatalf("test YAML does not parse to a mapping: %v\n%s", err, src)
	}
	return doc.Map()
}

// merged returns base with the top-level keys of each override replaced, as Ruby's
// Hash#merge does.
func merged(t *testing.T, base string, overrides ...string) *yamlord.Ordered {
	t.Helper()
	m := parseYAML(t, base)
	for _, override := range overrides {
		if strings.TrimSpace(override) != "" {
			m = m.Merge(parseYAML(t, override))
		}
	}
	return m
}

// deepMerged returns base with each override deep-merged over it.
func deepMerged(t *testing.T, base string, overrides ...string) *yamlord.Ordered {
	t.Helper()
	m := parseYAML(t, base)
	for _, override := range overrides {
		m = m.DeepMerge(parseYAML(t, override))
	}
	return m
}

// fakeWorld holds the fakes that a configuration test controls.
type fakeWorld struct {
	env     testsupport.FakeEnv
	files   testsupport.FakeFiles
	results map[string]localshell.Result
}

func newWorld() *fakeWorld {
	return &fakeWorld{
		env:   testsupport.FakeEnv{"RAILS_MASTER_KEY": "456", "VERSION": "missing"},
		files: testsupport.FakeFiles{},
		results: map[string]localshell.Result{
			"docker info --format '{{.Architecture}}'": {Stdout: "x86_64\n", Success: true},
		},
	}
}

func (w *fakeWorld) environment() Environment {
	sh := &localshell.Fake{Results: w.results}
	return Environment{
		Lookup:       w.env.Lookup,
		Shell:        sh,
		Git:          git.Git{Shell: sh},
		Files:        w.files,
		SecretFiles:  w.files,
		Substitution: secrets.Substitution{Shell: sh, Lookup: w.env.Lookup},
		WorkDir:      "/home/app/project",
		TempDir:      "/tmp",
		RandomHex:    func() string { return "0123456789abcdef" },
	}
}

func (w *fakeWorld) build(raw *yamlord.Ordered) (*Configuration, error) {
	return New(raw, Options{Env: w.environment()})
}

func (w *fakeWorld) buildDestination(raw *yamlord.Ordered, destination string) (*Configuration, error) {
	return New(raw, Options{Destination: destination, Env: w.environment()})
}

func mustBuild(t *testing.T, w *fakeWorld, raw *yamlord.Ordered) *Configuration {
	t.Helper()
	cfg, err := w.build(raw)
	if err != nil {
		t.Fatalf("configuration rejected: %v", err)
	}
	return cfg
}

func expectError(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("no error, want %q", want)
	}
	if err.Error() != want {
		t.Fatalf("error = %q\nwant    %q", err.Error(), want)
	}
}

func expectErrorContaining(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want one containing %q", err, want)
	}
}

func joined(items []string) string { return strings.Join(items, " ") }

// withSecrets stores a .kamal/secrets file, as with_test_secrets does.
func (w *fakeWorld) withSecrets(content string) *fakeWorld {
	w.files[".kamal/secrets"] = content
	return w
}
