package configuration

// Ports the host and role queries of lib/kamal/configuration.rb at Kamal 2.12.0.

import "github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"

// Roles returns the roles in name order.
func (c *Configuration) Roles() []*Role { return c.servers.Roles() }

// Role returns the named role, or nil.
func (c *Configuration) Role(name string) *Role {
	for _, role := range c.Roles() {
		if role.Name() == name {
			return role
		}
	}
	return nil
}

// Accessory returns the named accessory, or nil.
func (c *Configuration) Accessory(name string) *Accessory {
	for _, accessory := range c.accessories {
		if accessory.Name() == name {
			return accessory
		}
	}
	return nil
}

// AllHosts returns the hosts of every role and accessory, without duplicates.
func (c *Configuration) AllHosts() []string {
	var hosts []string
	for _, role := range c.Roles() {
		hosts = append(hosts, role.Hosts()...)
	}
	for _, accessory := range c.accessories {
		hosts = append(hosts, accessory.Hosts()...)
	}
	return uniq(hosts)
}

// HostRoles returns the roles that run on host.
func (c *Configuration) HostRoles(host string) []*Role {
	var roles []*Role
	for _, role := range c.Roles() {
		if utils.Contains(role.Hosts(), host) {
			roles = append(roles, role)
		}
	}
	return roles
}

// HostAccessories returns the accessories that run on host.
func (c *Configuration) HostAccessories(host string) []*Accessory {
	var accessories []*Accessory
	for _, accessory := range c.accessories {
		if utils.Contains(accessory.Hosts(), host) {
			accessories = append(accessories, accessory)
		}
	}
	return accessories
}

// AppHosts returns the hosts of every role, without duplicates.
func (c *Configuration) AppHosts() []string {
	var hosts []string
	for _, role := range c.Roles() {
		hosts = append(hosts, role.Hosts()...)
	}
	return uniq(hosts)
}

// PrimaryHost returns the first host of the primary role, or "".
func (c *Configuration) PrimaryHost() string {
	if role := c.PrimaryRole(); role != nil {
		return role.PrimaryHost()
	}
	return ""
}

// PrimaryRoleName returns primary_role, default web.
func (c *Configuration) PrimaryRoleName() string {
	if name := c.raw.At("primary_role"); name.Truthy() {
		return name.ToS()
	}
	return "web"
}

// PrimaryRole returns the primary role, or nil.
func (c *Configuration) PrimaryRole() *Role { return c.Role(c.PrimaryRoleName()) }

// AllowEmptyRoles reports whether roles without hosts are allowed.
func (c *Configuration) AllowEmptyRoles() bool { return c.raw.At("allow_empty_roles").Truthy() }

// ProxyRoles returns the roles that run behind kamal-proxy.
func (c *Configuration) ProxyRoles() []*Role {
	var roles []*Role
	for _, role := range c.Roles() {
		if role.RunningProxy() {
			roles = append(roles, role)
		}
	}
	return roles
}

// ProxyRoleNames returns the names of ProxyRoles.
func (c *Configuration) ProxyRoleNames() []string {
	var names []string
	for _, role := range c.ProxyRoles() {
		names = append(names, role.Name())
	}
	return names
}

// ProxyAccessories returns the accessories that run behind kamal-proxy.
func (c *Configuration) ProxyAccessories() []*Accessory {
	var accessories []*Accessory
	for _, accessory := range c.accessories {
		if accessory.RunningProxy() {
			accessories = append(accessories, accessory)
		}
	}
	return accessories
}

// ProxyHosts returns the hosts that run kamal-proxy.
func (c *Configuration) ProxyHosts() []string {
	var hosts []string
	for _, role := range c.ProxyRoles() {
		hosts = append(hosts, role.Hosts()...)
	}
	for _, accessory := range c.ProxyAccessories() {
		hosts = append(hosts, accessory.Hosts()...)
	}
	return uniq(hosts)
}
