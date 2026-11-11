package adapters

// Shared helpers of the ported adapter tests. A localshell.Fake answers each vendor
// command, as SecretAdapterTestCase#stub_ticks does in test/test_helper.rb.

import (
	"reflect"
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
)

func ok(stdout string) localshell.Result { return localshell.Result{Stdout: stdout, Success: true} }

var failed = localshell.Result{}

func noEnv(string) (string, bool) { return "", false }

// fetchWith looks up the adapter by name and fetches with a fake shell.
func fetchWith(t *testing.T, name string, results map[string]localshell.Result, secrets []string, account, from string) ([]Secret, error) {
	t.Helper()
	return fetchWithEnv(t, name, results, noEnv, secrets, account, from)
}

func fetchWithEnv(t *testing.T, name string, results map[string]localshell.Result, env func(string) (string, bool), secrets []string, account, from string) ([]Secret, error) {
	t.Helper()
	adapter, err := Lookup(name, Deps{Shell: &localshell.Fake{Results: results}, Lookup: env})
	if err != nil {
		t.Fatal(err)
	}
	return adapter.Fetch(secrets, account, from)
}

func assertSecrets(t *testing.T, want []Secret, got []Secret, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("secrets = %v, want %v", got, want)
	}
}

func assertFetchError(t *testing.T, want string, err error) {
	t.Helper()
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
}
