package adapters

// Ports lib/kamal/secrets/adapters/base.rb at Kamal 2.12.0.

import (
	"errors"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
)

// vendor is one vault: the steps that Base#fetch runs in order.
type vendor interface {
	requiresAccount() bool
	checkDependencies() error
	login(account string) error
	fetchSecrets(secrets []string, from, account string) ([]Secret, error)
}

// adapter runs a vendor's steps, as Base#fetch does.
type adapter struct{ vendor vendor }

func (a adapter) RequiresAccount() bool { return a.vendor.requiresAccount() }

// Fetch checks the account and the vendor tool, logs in, and fetches the secrets.
func (a adapter) Fetch(secrets []string, account, from string) ([]Secret, error) {
	if a.vendor.requiresAccount() && strings.TrimSpace(account) == "" {
		return nil, errors.New("Missing required option '--account'")
	}
	if err := a.vendor.checkDependencies(); err != nil {
		return nil, err
	}
	if err := a.vendor.login(account); err != nil {
		return nil, err
	}
	return a.vendor.fetchSecrets(secrets, from, account)
}

// prefixedSecrets puts from in front of each secret: "secret" and "KEY1" give
// "secret/KEY1". An empty from leaves the secrets as they are.
func prefixedSecrets(secrets []string, from string) []string {
	out := make([]string, len(secrets))
	for i, secret := range secrets {
		out[i] = secret
		if from != "" {
			out[i] = from + "/" + secret
		}
	}
	return out
}

// succeeds runs command and reports whether it exited with status 0, as a backtick
// call followed by $?.success? does.
func succeeds(shell localshell.Shell, command string) bool {
	result, err := shell.Run(command)
	return err == nil && result.Success
}

// rubySplit splits text at sep and drops the empty strings at the end, as Ruby's
// String#split does: "a/b/" gives ["a", "b"].
func rubySplit(text, sep string) []string {
	parts := strings.Split(text, sep)
	for len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}
