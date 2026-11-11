package configuration

// Ports lib/kamal/configuration/registry.rb at Kamal 2.12.0.

import (
	"strconv"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/secrets"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// Registry holds the image registry and its credentials. A credential written as a
// one-item list names a secret.
type Registry struct {
	config  *yamlord.Ordered
	secrets *secrets.Secrets
}

// newRegistry reads parent["registry"], where parent is the deploy file or an
// accessory.
func newRegistry(parent *yamlord.Ordered, sec *secrets.Secrets, context string) (*Registry, error) {
	config := parent.At("registry")
	if config.IsNull() {
		config = yamlord.NewMap(nil)
	}
	if err := validateRegistry(config, example("registry"), context); err != nil {
		return nil, err
	}
	return &Registry{config: config.Map(), secrets: sec}, nil
}

// Server returns the registry host, or "" for Docker Hub.
func (r *Registry) Server() string { return r.config.At("server").ToS() }

// Username returns the user name, looking up a secret when it is a list.
func (r *Registry) Username() (string, error) { return r.lookup("username") }

// Password returns the password, looking up a secret when it is a list.
func (r *Registry) Password() (string, error) { return r.lookup("password") }

// Local reports whether the registry runs on localhost on the deploy hosts.
func (r *Registry) Local() bool { return localRegistry.MatchString(r.Server()) }

// LocalPort returns the port of a local registry, as Ruby's
// server.split(":").last.to_i does.
func (r *Registry) LocalPort() (int, bool) {
	if !r.Local() {
		return 0, false
	}
	parts := strings.Split(r.Server(), ":")
	port, _ := strconv.Atoi(parts[len(parts)-1])
	return port, true
}

func (r *Registry) lookup(key string) (string, error) {
	value := r.config.At(key)
	if value.Kind() == yamlord.List {
		return r.secrets.Fetch(value.List()[0].ToS())
	}
	return value.ToS(), nil
}
