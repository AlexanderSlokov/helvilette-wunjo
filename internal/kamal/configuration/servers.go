package configuration

// Ports lib/kamal/configuration/servers.rb at Kamal 2.12.0.

import (
	"sort"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// Servers holds the roles of the deploy file.
type Servers struct {
	config *yamlord.Value
	roles  []*Role
}

func newServers(cfg *Configuration) (*Servers, error) {
	config := cfg.raw.At("servers")
	if err := validateServers(config, "servers"); err != nil {
		return nil, err
	}
	s := &Servers{config: config}
	for _, name := range roleNames(config) {
		role, err := newRole(name, cfg)
		if err != nil {
			return nil, err
		}
		s.roles = append(s.roles, role)
	}
	return s, nil
}

// roleNames returns web for a list of hosts, none for nil, and the sorted role names
// of a Hash.
func roleNames(config *yamlord.Value) []string {
	switch config.Kind() {
	case yamlord.List:
		return []string{"web"}
	case yamlord.Map:
		names := config.Map().Keys()
		sort.Strings(names)
		return names
	default:
		return nil
	}
}

// Roles returns the roles in name order.
func (s *Servers) Roles() []*Role { return s.roles }
