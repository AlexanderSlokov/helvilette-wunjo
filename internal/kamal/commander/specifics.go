// Package commander holds the state of one Kamal run: the configuration, the hosts
// and roles that --hosts and --roles select, the lock state, and the command
// builders.
//
// Ports lib/kamal/commander.rb and lib/kamal/commander/specifics.rb at Kamal 2.12.0.
package commander

import (
	"sort"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
)

// Specifics resolves the selected hosts and roles, with the primary role first.
type Specifics struct {
	cfg           *configuration.Configuration
	specificHosts []string
	specificRoles []*configuration.Role
	roles         []*configuration.Role
	hosts         []string
	primaryHost   string
	primaryRole   *configuration.Role
}

func newSpecifics(cfg *configuration.Configuration, hosts []string, roles []*configuration.Role) *Specifics {
	s := &Specifics{cfg: cfg, specificHosts: hosts, specificRoles: roles}
	s.roles, s.hosts = s.specifiedRoles(), s.specifiedHosts()
	s.primaryHost = s.findPrimaryHost()
	s.primaryRole = s.primaryOrFirstRole(s.RolesOn(s.primaryHost))
	sort.SliceStable(s.roles, func(i, j int) bool { return s.roles[i] == s.primaryRole && s.roles[j] != s.primaryRole })
	s.sortPrimaryRoleHostsFirst(s.hosts)
	return s
}

func (s *Specifics) findPrimaryHost() string {
	if len(s.specificHosts) > 0 {
		return s.specificHosts[0]
	}
	if role := s.primarySpecificRole(); role != nil {
		return role.PrimaryHost()
	}
	return s.cfg.PrimaryHost()
}

// Hosts returns the selected hosts, primary role hosts first.
func (s *Specifics) Hosts() []string { return s.hosts }

// Roles returns the selected roles, primary role first.
func (s *Specifics) Roles() []*configuration.Role { return s.roles }

// PrimaryHost returns the first selected host, or the primary host of the
// configuration.
func (s *Specifics) PrimaryHost() string { return s.primaryHost }

// PrimaryRole returns the primary role among the roles on the primary host.
func (s *Specifics) PrimaryRole() *configuration.Role { return s.primaryRole }

// RolesOn returns the selected roles that run on host.
func (s *Specifics) RolesOn(host string) []*configuration.Role {
	var roles []*configuration.Role
	for _, role := range s.roles {
		if utils.Contains(role.Hosts(), host) {
			roles = append(roles, role)
		}
	}
	return roles
}

// AppHosts returns the selected hosts that run the app, primary role hosts first.
func (s *Specifics) AppHosts() []string {
	hosts := intersect(s.cfg.AppHosts(), s.specifiedHosts())
	s.sortPrimaryRoleHostsFirst(hosts)
	return hosts
}

// ProxyHosts returns the selected hosts that run kamal-proxy.
func (s *Specifics) ProxyHosts() []string { return intersect(s.cfg.ProxyHosts(), s.specifiedHosts()) }

// AccessoryHosts returns the selected hosts that run accessories.
func (s *Specifics) AccessoryHosts() []string {
	var hosts []string
	for _, accessory := range s.cfg.Accessories() {
		hosts = append(hosts, accessory.Hosts()...)
	}
	return intersect(uniq(hosts), s.specifiedHosts())
}

func (s *Specifics) primarySpecificRole() *configuration.Role {
	if len(s.specificRoles) == 0 {
		return nil
	}
	return s.primaryOrFirstRole(s.specificRoles)
}

func (s *Specifics) primaryOrFirstRole(roles []*configuration.Role) *configuration.Role {
	for _, role := range roles {
		if role == s.cfg.PrimaryRole() {
			return role
		}
	}
	if len(roles) > 0 {
		return roles[0]
	}
	return nil
}

func (s *Specifics) specifiedRoles() []*configuration.Role {
	candidates := s.specificRoles
	if candidates == nil {
		candidates = s.cfg.Roles()
	}
	hosts := s.specificHosts
	if hosts == nil {
		hosts = s.cfg.AllHosts()
	}
	var roles []*configuration.Role
	for _, role := range candidates {
		if len(intersect(hosts, role.Hosts())) > 0 {
			roles = append(roles, role)
		}
	}
	return roles
}

func (s *Specifics) specifiedHosts() []string {
	hosts := s.specificHosts
	if hosts == nil {
		hosts = s.cfg.AllHosts()
	}
	var roleHosts []string
	for _, role := range s.specificRoles {
		roleHosts = append(roleHosts, role.Hosts()...)
	}
	if len(roleHosts) == 0 {
		return append([]string(nil), hosts...)
	}
	return intersect(hosts, roleHosts)
}

func (s *Specifics) sortPrimaryRoleHostsFirst(hosts []string) {
	rank := func(host string) int {
		for _, role := range s.RolesOn(host) {
			if role == s.primaryRole {
				return 0
			}
		}
		return 1
	}
	sort.SliceStable(hosts, func(i, j int) bool { return rank(hosts[i]) < rank(hosts[j]) })
}

// intersect keeps the items of a that b holds, in a's order, as Ruby's a & b.
func intersect(a, b []string) []string {
	var out []string
	for _, item := range a {
		if utils.Contains(b, item) && !utils.Contains(out, item) {
			out = append(out, item)
		}
	}
	return out
}

func uniq(items []string) []string { return intersect(items, items) }
