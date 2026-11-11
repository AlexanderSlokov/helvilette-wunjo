// Package adapters fetches secrets from vaults with each vendor's command-line tool,
// for `wunjo secrets fetch`.
//
// Ports lib/kamal/secrets/adapters.rb at Kamal 2.12.0. The port has the AWS Secrets
// Manager, GCP Secret Manager, and Doppler adapters, and the test adapter. The other
// six Kamal adapters wait on the roadmap, so their names are unknown here.
package adapters

import (
	"fmt"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
)

// Secret is one fetched secret, such as {"secret/KEY1", "VALUE1"}.
type Secret struct {
	Name  string
	Value string
}

// Adapter fetches secrets from one vault. Fetch returns the secrets in the order of
// the vendor's output, as Kamal's Hash keeps it.
type Adapter interface {
	RequiresAccount() bool
	Fetch(secrets []string, account, from string) ([]Secret, error)
}

// Deps are what the adapters read from the operator's machine: the shell that runs
// the vendor tools, and the environment.
type Deps struct {
	Shell  localshell.Shell
	Lookup func(name string) (string, bool)
}

// aliases are the short names that Adapters.lookup accepts.
var aliases = map[string]string{
	"1password": "one_password", "lastpass": "last_pass", "gcp": "gcp_secret_manager", "bitwarden-sm": "bitwarden_secrets_manager",
}

// Lookup returns the adapter for name, as Kamal::Secrets::Adapters.lookup does.
//
//	adapter, err := adapters.Lookup("gcp", adapters.Deps{Shell: localshell.System{}, Lookup: os.LookupEnv})
func Lookup(name string, deps Deps) (Adapter, error) {
	if alias, ok := aliases[strings.ToLower(name)]; ok {
		name = alias
	}
	newVendor, ok := vendors[camelize(name)]
	if !ok {
		return nil, fmt.Errorf("Unknown secrets adapter: %s", name)
	}
	return adapter{newVendor(deps)}, nil
}

var vendors = map[string]func(Deps) vendor{
	"AwsSecretsManager": func(d Deps) vendor { return awsSecretsManager{shell: d.Shell} },
	"GcpSecretManager":  func(d Deps) vendor { return gcpSecretManager{shell: d.Shell} },
	"Doppler":           func(d Deps) vendor { return doppler{shell: d.Shell, lookup: d.Lookup} },
	"Test":              func(Deps) vendor { return reversingVault{} },
}

// camelize turns "aws_secrets_manager" into "AwsSecretsManager", as ActiveSupport's
// String#camelize does for the class name.
func camelize(name string) string {
	words := strings.Split(name, "_")
	for i, word := range words {
		if word != "" {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, "")
}

// putSecret sets name to value, keeping the position of an earlier value, as a Ruby
// Hash does.
func putSecret(secrets []Secret, name, value string) []Secret {
	for i := range secrets {
		if secrets[i].Name == name {
			secrets[i].Value = value
			return secrets
		}
	}
	return append(secrets, Secret{Name: name, Value: value})
}
