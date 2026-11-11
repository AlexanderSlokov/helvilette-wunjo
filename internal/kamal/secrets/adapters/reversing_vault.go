package adapters

// Ports lib/kamal/secrets/adapters/test.rb at Kamal 2.12.0, the adapter that
// Kamal's CLI tests select with `--adapter test`.

import "strings"

// reversingVault returns each secret name reversed as its value, with LPAREN and
// RPAREN turned into parentheses first: "foo" gives "oof".
type reversingVault struct{}

func (reversingVault) requiresAccount() bool    { return true }
func (reversingVault) checkDependencies() error { return nil }
func (reversingVault) login(string) error       { return nil }

func (reversingVault) fetchSecrets(secrets []string, from, _ string) ([]Secret, error) {
	var results []Secret
	for _, secret := range prefixedSecrets(secrets, from) {
		value := strings.NewReplacer("LPAREN", "(", "RPAREN", ")").Replace(secret)
		results = putSecret(results, secret, reverse(value))
	}
	return results, nil
}

func reverse(text string) string {
	runes := []rune(text)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
