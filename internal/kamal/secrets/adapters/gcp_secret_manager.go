package adapters

// Ports lib/kamal/secrets/adapters/gcp_secret_manager.rb at Kamal 2.12.0.

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
)

// gcpSecretManager reads secrets with `gcloud secrets versions access`. The account
// is USER or USER|DELEGATION_CHAIN, where USER is "default" or an email and the chain
// lists the service accounts to impersonate, separated by commas.
type gcpSecretManager struct{ shell localshell.Shell }

// gcpSecret names one secret version: "some-project/item1/123".
type gcpSecret struct {
	project, name, version string
}

// gcpAccount is the user and the service accounts of --account.
type gcpAccount struct {
	user, serviceAccount string
}

func (gcpSecretManager) requiresAccount() bool { return true }

func (g gcpSecretManager) checkDependencies() error {
	if !succeeds(g.shell, "gcloud --version 2> /dev/null") {
		return errors.New("gcloud CLI is not installed")
	}
	return nil
}

func (g gcpSecretManager) login(string) error {
	if loggedIn, err := g.loggedIn(); err != nil || loggedIn {
		return err
	}
	_, _ = g.shell.Run("gcloud auth login")
	loggedIn, err := g.loggedIn()
	if err == nil && !loggedIn {
		err = errors.New("could not login to gcloud")
	}
	return err
}

func (g gcpSecretManager) loggedIn() (bool, error) {
	result, err := g.shell.Run("gcloud auth list --format=json")
	if err != nil {
		return false, err
	}
	var accounts []json.RawMessage
	if err := json.Unmarshal([]byte(result.Stdout), &accounts); err != nil {
		return false, fmt.Errorf("gcloud auth list: want a JSON array, got %.40q", result.Stdout)
	}
	return len(accounts) > 0, nil
}

func (g gcpSecretManager) fetchSecrets(secrets []string, from, account string) ([]Secret, error) {
	acct := parseGcpAccount(account)
	var results []Secret
	seen := map[string]bool{}
	for _, secret := range prefixedSecrets(secrets, from) {
		if seen[secret] {
			continue
		}
		seen[secret] = true
		item := parseGcpSecret(secret)
		value, err := g.fetchSecret(item, acct)
		if err != nil {
			return nil, err
		}
		results = putSecret(results, item.project+"/"+item.name, value)
	}
	return results, nil
}

func (g gcpSecretManager) fetchSecret(item gcpSecret, acct gcpAccount) (string, error) {
	result, err := g.shell.Run(gcpAccessCommand(item, acct))
	if err != nil {
		return "", err
	}
	var version struct {
		Payload struct {
			Data string `json:"data"`
		} `json:"payload"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Stdout)), &version); err != nil {
		return "", fmt.Errorf("gcloud secrets versions access: want JSON with payload.data, got %.40q", result.Stdout)
	}
	if !result.Success {
		return "", fmt.Errorf("Could not read %s/%s from Google Secret Manager", item.project, item.name)
	}
	data, err := base64.StdEncoding.DecodeString(version.Payload.Data)
	return string(data), err
}

func gcpAccessCommand(item gcpSecret, acct gcpAccount) string {
	words := []string{"gcloud", "secrets versions access " + shell.Escape(item.version) + " --secret=" + shell.Escape(item.name)}
	if item.project != "default" {
		words = append(words, "--project="+shell.Escape(item.project))
	}
	if acct.user != "default" {
		words = append(words, "--account="+shell.Escape(acct.user))
	}
	if acct.serviceAccount != "" {
		words = append(words, "--impersonate-service-account="+shell.Escape(acct.serviceAccount))
	}
	return strings.Join(append(words, "--format=json"), " ")
}

// parseGcpSecret reads [project/]name[/version]: the project defaults to "default"
// and the version to "latest".
func parseGcpSecret(secret string) gcpSecret {
	parts := rubySplit(secret, "/")
	if len(parts) == 1 {
		parts = append([]string{"default"}, parts...)
	}
	parts = append(parts, "", "", "")
	item := gcpSecret{project: parts[0], name: parts[1], version: parts[2]}
	if item.version == "" {
		item.version = "latest"
	}
	return item
}

func parseGcpAccount(account string) gcpAccount {
	user, serviceAccount, _ := strings.Cut(account, "|")
	return gcpAccount{user: user, serviceAccount: serviceAccount}
}
