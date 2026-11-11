package adapters

// Ports test/secrets/gcp_secret_manager_adapter_test.rb at Kamal 2.12.0. The Ruby
// tests run the CLI with --account default unless a test names another account.

import (
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
)

const (
	gcloudVersion  = "gcloud --version 2> /dev/null"
	gcloudAuthList = "gcloud auth list --format=json"
)

// gcpPayloads are "secret1", "secret2", and "secret3" in base64, as stub_items has.
var gcpPayloads = []string{"c2VjcmV0MQ==", "c2VjcmV0Mg==", "c2VjcmV0Mw=="}

func gcpAuthenticated() map[string]localshell.Result {
	return map[string]localshell.Result{gcloudVersion: ok(""), gcloudAuthList: ok(`[{"account": "email@example.com", "status": "ACTIVE"}]`)}
}

func gcpVersionOutput(data string) localshell.Result {
	return ok(`{"name": "projects/000000001/secrets/item1/versions/1", "payload": {"data": "` + data + `", "dataCrc32c": "1846998209"}}`)
}

// stubGcpItem stubs item<n+1>, as stub_items does: the flags follow the secret in
// the order project, account, and service account.
func stubGcpItem(results map[string]localshell.Result, n int, version, flags string) {
	command := "gcloud secrets versions access " + version + " --secret=item" + string(rune('1'+n)) + flags + " --format=json"
	results[command] = gcpVersionOutput(gcpPayloads[n])
}

func TestGcpFetch(t *testing.T) {
	results := gcpAuthenticated()
	results["gcloud secrets versions access latest --secret=mypassword --format=json"] = gcpVersionOutput("c2VjcmV0MTIz")
	got, err := fetchWith(t, "gcp_secret_manager", results, []string{"mypassword"}, "default", "")
	assertSecrets(t, []Secret{{"default/mypassword", "secret123"}}, got, err)
}

func TestGcpFetchUnauthenticated(t *testing.T) {
	results := map[string]localshell.Result{gcloudVersion: ok(""), gcloudAuthList: ok("[]"), "gcloud auth login": ok(`{"expired": false, "valid": true}`)}
	_, err := fetchWith(t, "gcp_secret_manager", results, []string{"mypassword"}, "default", "")
	assertFetchError(t, "could not login to gcloud", err)
}

func TestGcpFetchWithFrom(t *testing.T) {
	results := gcpAuthenticated()
	for n := range 3 {
		stubGcpItem(results, n, "latest", " --project=other-project")
	}
	got, err := fetchWith(t, "gcp_secret_manager", results, []string{"item1", "item2", "item3"}, "default", "other-project")
	assertSecrets(t, []Secret{{"other-project/item1", "secret1"}, {"other-project/item2", "secret2"}, {"other-project/item3", "secret3"}}, got, err)
}

func TestGcpFetchWithMultipleProjects(t *testing.T) {
	results := gcpAuthenticated()
	for n, project := range []string{"some-project", "project-confidence", "manhattan-project"} {
		stubGcpItem(results, n, "latest", " --project="+project)
	}
	got, err := fetchWith(t, "gcp_secret_manager", results, []string{"some-project/item1", "project-confidence/item2", "manhattan-project/item3"}, "default", "")
	assertSecrets(t, []Secret{{"some-project/item1", "secret1"}, {"project-confidence/item2", "secret2"}, {"manhattan-project/item3", "secret3"}}, got, err)
}

func TestGcpFetchWithAccounts(t *testing.T) {
	cases := map[string]string{
		"default":                          " --project=some-project",
		"email@example.com":                " --project=some-project --account=email@example.com",
		"default|service-user@example.com": " --project=some-project --impersonate-service-account=service-user@example.com",
		"user@example.com|service-user@example.com,service-user2@example.com": " --project=some-project --account=user@example.com" +
			" --impersonate-service-account=service-user@example.com,service-user2@example.com",
		"email@example.com|service-user@example.com": " --project=some-project --account=email@example.com --impersonate-service-account=service-user@example.com",
	}
	for account, flags := range cases {
		results := gcpAuthenticated()
		stubGcpItem(results, 0, "123", flags)
		got, err := fetchWith(t, "gcp_secret_manager", results, []string{"some-project/item1/123"}, account, "")
		assertSecrets(t, []Secret{{"some-project/item1", "secret1"}}, got, err)
	}
}

func TestGcpFetchWithoutCLIInstalled(t *testing.T) {
	_, err := fetchWith(t, "gcp_secret_manager", map[string]localshell.Result{gcloudVersion: failed}, []string{"item1"}, "default", "")
	assertFetchError(t, "gcloud CLI is not installed", err)
}

func TestParseGcpSecret(t *testing.T) {
	cases := map[string]gcpSecret{
		"my-secret": {"default", "my-secret", "latest"}, "default/my-secret": {"default", "my-secret", "latest"},
		"default/my-secret/latest": {"default", "my-secret", "latest"}, "some-project/my-secret/123": {"some-project", "my-secret", "123"},
	}
	for secret, want := range cases {
		if got := parseGcpSecret(secret); got != want {
			t.Errorf("parseGcpSecret(%q) = %+v, want %+v", secret, got, want)
		}
	}
}
