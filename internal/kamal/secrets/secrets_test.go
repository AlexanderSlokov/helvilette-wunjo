package secrets

import (
	"errors"
	"sync"
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/kamalerr"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/testsupport"
)

func newSecrets(files FakeFiles, destination string, sh localshell.Shell, env testsupport.FakeEnv) *Secrets {
	if sh == nil {
		sh = &localshell.Fake{}
	}
	return New(Options{
		Destination:  destination,
		Files:        files,
		Substitution: Substitution{Shell: sh, Lookup: env.Lookup},
	})
}

func mustFetch(t *testing.T, s *Secrets, key string) string {
	t.Helper()
	value, err := s.Fetch(key)
	if err != nil {
		t.Fatalf("Fetch(%q): %v", key, err)
	}
	return value
}

func TestFetch(t *testing.T) {
	s := newSecrets(FakeFiles{".kamal/secrets": "SECRET=ABC"}, "", nil, nil)
	if got := mustFetch(t, s, "SECRET"); got != "ABC" {
		t.Fatalf("got %q", got)
	}
}

// countingShell counts how often a command runs, for the load-once test.
type countingShell struct {
	mu    sync.Mutex
	calls int
}

func (c *countingShell) Run(string) (localshell.Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return localshell.Result{Stdout: "ABC\n", Success: true}, nil
}

func TestAllFromConcurrentCallersLoadsOnce(t *testing.T) {
	sh := &countingShell{}
	s := newSecrets(FakeFiles{".kamal/secrets": "SECRET=$(echo loaded >> loads; echo ABC)"}, "", sh, nil)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			if all, err := s.All(); err != nil || all.At("SECRET").ToS() != "ABC" {
				t.Errorf("All() = %v, %v", all, err)
			}
		})
	}
	wg.Wait()
	if sh.calls != 1 {
		t.Fatalf("command ran %d times, want 1", sh.calls)
	}
}

func TestHas(t *testing.T) {
	s := newSecrets(FakeFiles{".kamal/secrets": "SECRET1=ABC"}, "", nil, nil)
	if !s.Has("SECRET1") || s.Has("SECRET2") {
		t.Fatal("Has gave the wrong answer")
	}
}

func TestCommandInterpolation(t *testing.T) {
	sh := &localshell.Fake{Results: map[string]localshell.Result{"echo ABC": {Stdout: "ABC\n", Success: true}}}
	s := newSecrets(FakeFiles{".kamal/secrets": "SECRET=$(echo ABC)"}, "", sh, nil)
	if got := mustFetch(t, s, "SECRET"); got != "ABC" {
		t.Fatalf("got %q", got)
	}
}

func TestVariableReferences(t *testing.T) {
	s := newSecrets(FakeFiles{".kamal/secrets": "SECRET1=ABC\nSECRET2=${SECRET1}DEF"}, "", nil, nil)
	if mustFetch(t, s, "SECRET1") != "ABC" || mustFetch(t, s, "SECRET2") != "ABCDEF" {
		t.Fatal("variable reference did not expand")
	}
}

func TestEnvReferences(t *testing.T) {
	s := newSecrets(FakeFiles{".kamal/secrets": "SECRET1=$SECRET1"}, "", nil, testsupport.FakeEnv{"SECRET1": "ABC"})
	if got := mustFetch(t, s, "SECRET1"); got != "ABC" {
		t.Fatalf("got %q", got)
	}
}

func TestSecretsFileValueOverridesEnv(t *testing.T) {
	s := newSecrets(FakeFiles{".kamal/secrets": "SECRET1=DEF"}, "", nil, testsupport.FakeEnv{"SECRET1": "ABC"})
	if got := mustFetch(t, s, "SECRET1"); got != "DEF" {
		t.Fatalf("got %q", got)
	}
}

func TestDestinations(t *testing.T) {
	files := FakeFiles{".kamal/secrets.dest": "SECRET=DEF", ".kamal/secrets": "SECRET=ABC", ".kamal/secrets-common": "SECRET=GHI\nSECRET2=JKL"}
	want := map[string]string{"": "ABC", "dest": "DEF", "nodest": "GHI"}
	for destination, secret := range want {
		s := newSecrets(files, destination, nil, nil)
		if mustFetch(t, s, "SECRET") != secret || mustFetch(t, s, "SECRET2") != "JKL" {
			t.Errorf("destination %q: SECRET=%s SECRET2=%s", destination, mustFetch(t, s, "SECRET"), mustFetch(t, s, "SECRET2"))
		}
	}
}

func TestNoSecretsFiles(t *testing.T) {
	cases := map[string]string{
		"":     "Secret 'SECRET' not found, no secret files (.kamal/secrets-common, .kamal/secrets) provided",
		"dest": "Secret 'SECRET' not found, no secret files (.kamal/secrets-common, .kamal/secrets.dest) provided",
	}
	for destination, want := range cases {
		_, err := newSecrets(FakeFiles{}, destination, nil, nil).Fetch("SECRET")
		var configErr *kamalerr.ConfigurationError
		if !errors.As(err, &configErr) || err.Error() != want {
			t.Errorf("destination %q: error = %v", destination, err)
		}
	}
}

func TestSecretNotFoundNamesFiles(t *testing.T) {
	_, err := newSecrets(FakeFiles{".kamal/secrets": "A=1"}, "", nil, nil).Fetch("SECRET")
	if err == nil || err.Error() != "Secret 'SECRET' not found in .kamal/secrets" {
		t.Fatalf("error = %v", err)
	}
}

func TestCustomSecretsPath(t *testing.T) {
	files := FakeFiles{"custom/path/secrets": "SECRET=BASE", "custom/path/secrets.prod": "SECRET=PROD", "custom/path/secrets-common": "COMMON=SHARED"}
	base := New(Options{SecretsPath: "custom/path/secrets", Files: files})
	prod := New(Options{SecretsPath: "custom/path/secrets", Destination: "prod", Files: files})
	if mustFetch(t, base, "SECRET") != "BASE" || mustFetch(t, prod, "SECRET") != "PROD" || mustFetch(t, base, "COMMON") != "SHARED" {
		t.Fatal("custom secrets path read the wrong files")
	}
}

func TestCustomSecretsPathErrorMessage(t *testing.T) {
	_, err := New(Options{SecretsPath: "custom/path/secrets", Files: FakeFiles{}}).Fetch("SECRET")
	want := "Secret 'SECRET' not found, no secret files (custom/path/secrets-common, custom/path/secrets) provided"
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v", err)
	}
}
