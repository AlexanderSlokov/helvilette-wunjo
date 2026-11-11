package adapters

// Ports lib/kamal/secrets/adapters/doppler.rb at Kamal 2.12.0.

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
)

// doppler reads secrets with `doppler secrets get`. The project and config come from
// --from=project/config, unless DOPPLER_TOKEN holds a service token.
type doppler struct {
	shell  localshell.Shell
	lookup func(name string) (string, bool)
}

func (doppler) requiresAccount() bool { return false }

func (d doppler) checkDependencies() error {
	if !succeeds(d.shell, "doppler --version 2> /dev/null") {
		return errors.New("Doppler CLI is not installed")
	}
	return nil
}

func (d doppler) login(string) error {
	if succeeds(d.shell, "doppler me --json 2> /dev/null") || succeeds(d.shell, "doppler login -y") {
		return nil
	}
	return errors.New("Failed to login to Doppler")
}

func (d doppler) fetchSecrets(secrets []string, from, _ string) ([]Secret, error) {
	prefixed := prefixedSecrets(secrets, from)
	flags, err := d.projectFlags(prefixed)
	if err != nil {
		return nil, err
	}
	result, err := d.shell.Run("doppler secrets get " + strings.Join(secretNames(prefixed), " ") + " --json " + flags)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, fmt.Errorf("Could not read %s from Doppler", rubyArray(prefixed))
	}
	return computedValues([]byte(result.Stdout))
}

// projectFlags returns "-p <project> -c <config>" from the first secret, or nothing
// with a service token, which names its own project and config.
func (d doppler) projectFlags(secrets []string) (string, error) {
	if d.serviceTokenSet() {
		return "", nil
	}
	var parts []string
	if len(secrets) > 0 {
		parts = rubySplit(secrets[0], "/")
	}
	if len(parts) < 2 {
		return "", errors.New("Missing project or config from '--from=project/config' option")
	}
	return "-p " + shell.Escape(parts[0]) + " -c " + shell.Escape(parts[1]), nil
}

func (d doppler) serviceTokenSet() bool {
	token, ok := d.lookup("DOPPLER_TOKEN")
	return ok && strings.HasPrefix(token, "dp.st")
}

// secretNames returns the last part of each secret, escaped for the shell:
// "my-project/prd/SECRET1" gives "SECRET1".
func secretNames(secrets []string) []string {
	names := make([]string, len(secrets))
	for i, secret := range secrets {
		parts := rubySplit(secret, "/")
		if len(parts) > 0 {
			names[i] = shell.Escape(parts[len(parts)-1])
		}
	}
	return names
}

// computedValues reads the "computed" value of each secret in Doppler's output.
func computedValues(output []byte) ([]Secret, error) {
	fields, err := objectFields(output)
	if err != nil {
		return nil, fmt.Errorf("Doppler output is not a JSON object: %w", err)
	}
	var results []Secret
	for _, field := range fields {
		var item struct {
			Computed string `json:"computed"`
		}
		if err := json.Unmarshal(field.Value, &item); err != nil {
			return nil, fmt.Errorf("Doppler secret %s: want {\"computed\": \"...\"}: %w", field.Key, err)
		}
		results = putSecret(results, field.Key, item.Computed)
	}
	return results, nil
}

// rubyArray writes items as Ruby's Array#to_s does: ["a", "b"].
func rubyArray(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = utils.RubyDump(item)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
