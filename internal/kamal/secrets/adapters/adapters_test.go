package adapters

// Tests the lookup of lib/kamal/secrets/adapters.rb, the base checks of
// adapters/base.rb, and the test adapter, at Kamal 2.12.0.

import (
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
)

func TestLookupAcceptsKamalNamesAndAliases(t *testing.T) {
	for _, name := range []string{"aws_secrets_manager", "AwsSecretsManager", "gcp", "GCP", "gcp_secret_manager", "doppler", "test"} {
		if _, err := Lookup(name, Deps{Shell: &localshell.Fake{}, Lookup: noEnv}); err != nil {
			t.Errorf("Lookup(%q): %v", name, err)
		}
	}
}

// Kamal renames an alias before it looks up the class, so the message names
// one_password for --adapter 1password.
func TestLookupOfAnAdapterThatIsNotPorted(t *testing.T) {
	cases := map[string]string{"1password": "one_password", "bitwarden": "bitwarden", "lastpass": "last_pass", "vault": "vault"}
	for name, shown := range cases {
		_, err := Lookup(name, Deps{Shell: &localshell.Fake{}, Lookup: noEnv})
		assertFetchError(t, "Unknown secrets adapter: "+shown, err)
	}
}

func TestFetchRequiresAnAccountWhenTheAdapterDoes(t *testing.T) {
	_, err := fetchWith(t, "gcp", nil, []string{"item1"}, " ", "")
	assertFetchError(t, "Missing required option '--account'", err)
}

func TestReversingVault(t *testing.T) {
	got, err := fetchWith(t, "test", nil, []string{"foo", "fooLPARENbarRPAREN", "foo"}, "myaccount", "")
	assertSecrets(t, []Secret{{"foo", "oof"}, {"fooLPARENbarRPAREN", ")rab(oof"}}, got, err)
	got, err = fetchWith(t, "test", nil, []string{"foo"}, "myaccount", "vault")
	assertSecrets(t, []Secret{{"vault/foo", "oof/tluav"}}, got, err)
}

func TestObjectFieldsKeepsTheOrder(t *testing.T) {
	fields, err := objectFields([]byte(`{"b": 1, "a": {"x": [1, 2]}}`))
	if err != nil || len(fields) != 2 || fields[0].Key != "b" || fields[1].Key != "a" || stringifyJSON(fields[1].Value) != `{"x":[1,2]}` {
		t.Fatalf("fields = %+v, err = %v", fields, err)
	}
	if _, err := objectFields([]byte(`[1]`)); err == nil {
		t.Fatal("objectFields accepted an array")
	}
}

func TestRubySplitDropsTrailingEmptyParts(t *testing.T) {
	if got := rubySplit("a//b/", "/"); len(got) != 3 || got[1] != "" || got[2] != "b" {
		t.Fatalf("rubySplit = %q", got)
	}
}
