package configuration

// Ports lib/kamal/configuration/ssh.rb at Kamal 2.12.0.

import (
	"fmt"
	"io"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/secrets"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// SSH holds the settings for connecting to hosts.
type SSH struct {
	config  *yamlord.Ordered
	secrets *secrets.Secrets
	stderr  io.Writer
}

// SSHProxy is a jump host ("user@host") or a proxy command. At most one is set.
type SSHProxy struct {
	Jump    string
	Command string
}

func newSSH(config *yamlord.Value, sec *secrets.Secrets, stderr io.Writer) (*SSH, error) {
	if config.IsNull() {
		config = yamlord.NewMap(nil)
	}
	if err := (validator{context: "ssh"}).againstExample(config, example("ssh")); err != nil {
		return nil, err
	}
	return &SSH{config: config.Map(), secrets: sec, stderr: stderr}, nil
}

// User returns the SSH user, default root.
func (s *SSH) User() string { return s.config.Fetch("user", yamlord.NewString("root")).ToS() }

// Port returns the SSH port, default 22.
func (s *SSH) Port() int64 { return s.config.Fetch("port", yamlord.NewInt(22)).Int() }

// Proxy returns the jump host or proxy command, and false when neither is set. A jump
// host without a user connects as root.
func (s *SSH) Proxy() (SSHProxy, bool) {
	if proxy := s.config.At("proxy"); proxy.Truthy() {
		jump := proxy.ToS()
		if !strings.Contains(jump, "@") {
			jump = "root@" + jump
		}
		return SSHProxy{Jump: jump}, true
	}
	if command := s.config.At("proxy_command"); command.Truthy() {
		return SSHProxy{Command: command.ToS()}, true
	}
	return SSHProxy{}, false
}

// KeysOnly returns the keys_only setting, or nil.
func (s *SSH) KeysOnly() *yamlord.Value { return s.config.At("keys_only") }

// Keys returns the key file paths, or nil.
func (s *SSH) Keys() *yamlord.Value { return s.config.At("keys") }

// KeyData returns the private keys. Each entry names a secret; an entry that is not a
// secret is the key itself, which Kamal 2.12.0 deprecates with a warning.
func (s *SSH) KeyData() ([]string, bool, error) {
	entries := s.config.At("key_data")
	if !entries.Truthy() {
		return nil, false, nil
	}
	var keys []string
	for _, entry := range entries.StringList() {
		keys = append(keys, s.keyFor(entry))
	}
	return keys, true, nil
}

func (s *SSH) keyFor(entry string) string {
	if s.secrets.Has(entry) {
		value, _ := s.secrets.Fetch(entry)
		return value
	}
	fmt.Fprintln(s.stderr, "Inline key_data usage is deprecated and will be removed in Kamal 3. Please store your key_data in a secret.")
	return entry
}

// Config returns the ssh_config setting: true, false, a path, a list of paths, or
// nil.
func (s *SSH) Config() *yamlord.Value { return s.config.At("config") }

// ForwardAgent returns the forward_agent setting, or nil.
func (s *SSH) ForwardAgent() *yamlord.Value { return s.config.At("forward_agent") }

// LogLevel returns the SSH log level, default fatal.
func (s *SSH) LogLevel() string {
	return s.config.Fetch("log_level", yamlord.NewSymbol("fatal")).ToS()
}

// LoggerLevel returns the Ruby Logger level number of LogLevel: debug 0 to unknown 5.
func (s *SSH) LoggerLevel() int {
	levels := map[string]int{"debug": 0, "info": 1, "warn": 2, "error": 3, "fatal": 4, "unknown": 5}
	if level, ok := levels[strings.ToLower(s.LogLevel())]; ok {
		return level
	}
	return 4
}

// ToH returns the options that `kamal config` prints, in Kamal's order: user, port,
// proxy, keepalive, keepalive_interval, keys_only, keys, key_data, config,
// forward_agent, and log_level. Unset options are left out.
func (s *SSH) ToH() (*yamlord.Ordered, error) {
	out := yamlord.OrderedOf(
		yamlord.Pair{Key: "user", Value: yamlord.NewString(s.User())},
		yamlord.Pair{Key: "port", Value: yamlord.NewInt(s.Port())},
	)
	if proxy, ok := s.Proxy(); ok {
		out.Set("proxy", yamlord.NewString(proxy.Jump+proxy.Command))
	}
	out.Set("keepalive", yamlord.NewBool(true))
	out.Set("keepalive_interval", yamlord.NewInt(30))
	if err := s.setOptional(out); err != nil {
		return nil, err
	}
	out.Set("log_level", yamlord.NewSymbol(s.LogLevel()))
	return out, nil
}

func (s *SSH) setOptional(out *yamlord.Ordered) error {
	setIfPresent(out, "keys_only", s.KeysOnly())
	setIfPresent(out, "keys", s.Keys())
	keys, ok, err := s.KeyData()
	if err != nil {
		return err
	}
	if ok {
		out.Set("key_data", yamlord.Strings(keys...))
	}
	setIfPresent(out, "config", s.Config())
	setIfPresent(out, "forward_agent", s.ForwardAgent())
	return nil
}

// setIfPresent sets key unless value is nil, as Hash#compact keeps false.
func setIfPresent(out *yamlord.Ordered, key string, value *yamlord.Value) {
	if !value.IsNull() {
		out.Set(key, value)
	}
}
