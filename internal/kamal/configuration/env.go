package configuration

// Ports lib/kamal/configuration/env.rb and env/tag.rb at Kamal 2.12.0.

import (
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/envfile"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/secrets"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// Env holds clear environment variables and the names of secret ones.
type Env struct {
	clear      *yamlord.Ordered
	secretKeys []string
	secrets    *secrets.Secrets
	context    string
}

// newEnv validates config and builds an Env. A config without clear, secret, or
// tags keys is all clear variables.
func newEnv(config *yamlord.Value, sec *secrets.Secrets, context string) (*Env, error) {
	if config.IsNull() {
		config = yamlord.NewMap(nil)
	}
	if err := validateEnv(config, example("env"), context); err != nil {
		return nil, err
	}
	m := config.Map()
	clear := m
	if m.Has("secret") || m.Has("tags") {
		clear = yamlord.NewOrdered()
	}
	if m.Has("clear") {
		clear = m.At("clear").Map()
	}
	return &Env{clear: clear, secretKeys: m.At("secret").StringList(), secrets: sec, context: context}, nil
}

// Clear returns the clear variables in file order.
func (e *Env) Clear() *yamlord.Ordered { return e.clear }

// SecretKeys returns the secret names, such as "DB_PASSWORD:MYSQL_PASSWORD".
func (e *Env) SecretKeys() []string { return e.secretKeys }

// ClearArgs returns --env flags for the clear variables.
func (e *Env) ClearArgs() shell.Line {
	return utils.Argumentize("--env", utils.AttrsOf(e.clear), false)
}

// SecretsFile returns the env file for the secret variables, with values looked up.
func (e *Env) SecretsFile() (string, error) {
	vars, err := e.aliasedSecrets()
	if err != nil {
		return "", err
	}
	return envfile.Render(vars), nil
}

// Merge returns an Env with the clear variables of both, other winning, and the
// union of the secret names.
func (e *Env) Merge(other *Env) *Env {
	return &Env{clear: e.clear.Merge(other.clear), secretKeys: union(e.secretKeys, other.secretKeys), secrets: e.secrets, context: "env"}
}

// ToH returns the clear and secret variables together, as Env#to_h does.
func (e *Env) ToH() (*yamlord.Ordered, error) {
	vars, err := e.aliasedSecrets()
	if err != nil {
		return nil, err
	}
	out := e.clear.Clone()
	for _, v := range vars {
		out.Set(v.Key, yamlord.NewString(v.Value))
	}
	return out, nil
}

// aliasedSecrets looks up each secret. "NAME:SECRET" sets NAME from SECRET.
func (e *Env) aliasedSecrets() ([]envfile.Var, error) {
	var vars []envfile.Var
	for _, key := range e.secretKeys {
		name, secret, found := strings.Cut(key, ":")
		if !found {
			secret = name
		}
		value, err := e.secrets.Fetch(secret)
		if err != nil {
			return nil, err
		}
		vars = setVar(vars, envfile.Var{Key: name, Value: value})
	}
	return vars, nil
}

// setVar replaces an existing variable in place, as a Ruby Hash assignment does.
func setVar(vars []envfile.Var, v envfile.Var) []envfile.Var {
	for i := range vars {
		if vars[i].Key == v.Key {
			vars[i] = v
			return vars
		}
	}
	return append(vars, v)
}

func union(a, b []string) []string {
	var out []string
	for _, item := range append(append([]string(nil), a...), b...) {
		if !utils.Contains(out, item) {
			out = append(out, item)
		}
	}
	return out
}

// EnvTag is a named set of variables that hosts tagged with the name receive.
type EnvTag struct {
	Name   string
	config *yamlord.Value
	sec    *secrets.Secrets
}

// Env returns the tag's variables.
func (t *EnvTag) Env() (*Env, error) { return newEnv(t.config, t.sec, "env") }
