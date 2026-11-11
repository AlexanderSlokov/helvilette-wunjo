package configuration

// Ports lib/kamal/configuration/sshkit.rb at Kamal 2.12.0.

import (
	"fmt"
	"strconv"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// Sshkit holds the connection pool settings.
type Sshkit struct {
	config *yamlord.Ordered
}

func newSshkit(config *yamlord.Value) (*Sshkit, error) {
	if config.IsNull() {
		config = yamlord.NewMap(nil)
	}
	if err := (validator{context: "sshkit"}).againstExample(config, example("sshkit")); err != nil {
		return nil, err
	}
	return &Sshkit{config: config.Map()}, nil
}

// MaxConcurrentStarts returns how many SSH connections start at once, default 30.
func (s *Sshkit) MaxConcurrentStarts() int64 { return s.intOr("max_concurrent_starts", 30) }

// PoolIdleTimeout returns the seconds before an idle connection closes, default 900.
func (s *Sshkit) PoolIdleTimeout() int64 { return s.intOr("pool_idle_timeout", 900) }

// DNSRetries returns how often a DNS failure retries, default 3, as Integer() of the
// setting.
func (s *Sshkit) DNSRetries() (int64, error) {
	value := s.config.Fetch("dns_retries", yamlord.NewInt(3))
	if value.Kind() == yamlord.Int {
		return value.Int(), nil
	}
	n, err := strconv.ParseInt(value.ToS(), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid value for Integer(): %s", value.Inspect())
	}
	return n, nil
}

// ToH returns the raw settings.
func (s *Sshkit) ToH() *yamlord.Ordered { return s.config }

func (s *Sshkit) intOr(key string, fallback int64) int64 {
	if value, ok := s.config.Get(key); ok {
		return value.Int()
	}
	return fallback
}
