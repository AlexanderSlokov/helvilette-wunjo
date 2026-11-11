package configuration

// Ports the ensure_* checks of lib/kamal/configuration.rb at Kamal 2.12.0.

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/kamalerr"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
)

// KamalVersion is the Kamal release that Wunjo ports, which minimum_version compares
// against.
const KamalVersion = "2.12.0"

// ErrDestinationRequired reports require_destination without a destination. Kamal
// raises it as an ArgumentError, outside ConfigurationError.
var ErrDestinationRequired = errors.New("You must specify a destination")

var hooksOutputLevels = []string{"quiet", "verbose"}

func (c *Configuration) ensureValid() error {
	checks := []func() error{
		c.ensureDestinationIfRequired, c.ensureRequiredKeysPresent, c.ensureValidKamalVersion,
		c.ensureRetainContainersValid, c.ensureValidServiceName, c.ensureNoTraefikRebootHooks,
		c.ensureOneHostForSSLRoles, c.ensureUniqueHostsForSSLRoles,
		c.ensureLocalRegistryRemoteBuilderHasSSHURL, c.ensureNoConflictingProxyRuns, c.ensureValidHooksOutput,
	}
	for _, check := range checks {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Configuration) ensureDestinationIfRequired() error {
	if c.RequireDestination() && c.destination == "" {
		return ErrDestinationRequired
	}
	return nil
}

func (c *Configuration) ensureRequiredKeysPresent() error {
	for _, key := range []string{"service", "registry"} {
		if !c.raw.At(key).Present() {
			return kamalerr.Configuration("Missing required configuration for " + key)
		}
	}
	if strings.TrimSpace(c.Image()) == "" {
		return kamalerr.Configuration("Missing required configuration for image")
	}
	if c.raw.At("servers").IsNull() {
		return c.ensureAccessoriesWithoutServers()
	}
	return c.ensureRolesHaveHosts()
}

func (c *Configuration) ensureAccessoriesWithoutServers() error {
	if !c.raw.At("accessories").Present() {
		return kamalerr.Configuration("No servers or accessories specified")
	}
	return nil
}

func (c *Configuration) ensureRolesHaveHosts() error {
	primary := c.PrimaryRole()
	if primary == nil {
		return kamalerr.Configuration(fmt.Sprintf("The primary_role %s isn't defined", c.PrimaryRoleName()))
	}
	if len(primary.Hosts()) == 0 {
		return kamalerr.Configuration(fmt.Sprintf("No servers specified for the %s primary_role", primary.Name()))
	}
	if c.AllowEmptyRoles() {
		return nil
	}
	for _, role := range c.Roles() {
		if len(role.Hosts()) == 0 {
			return kamalerr.Configuration(fmt.Sprintf("No servers specified for the %s role. You can ignore this with allow_empty_roles: true", role.Name()))
		}
	}
	return nil
}

var serviceName = regexp.MustCompile(`(?i)^[a-z0-9_-]+$`)

func (c *Configuration) ensureValidServiceName() error {
	if !serviceName.MatchString(c.Service()) {
		return kamalerr.Configuration("Service name can only include alphanumeric characters, hyphens, and underscores")
	}
	return nil
}

func (c *Configuration) ensureValidKamalVersion() error {
	minimum := c.MinimumVersion()
	if minimum != "" && utils.OlderVersion(KamalVersion, minimum) {
		return kamalerr.Configuration(fmt.Sprintf("Current version is %s, minimum required is %s", KamalVersion, minimum))
	}
	return nil
}

func (c *Configuration) ensureRetainContainersValid() error {
	if c.RetainContainers() < 1 {
		return kamalerr.Configuration("Must retain at least 1 container")
	}
	return nil
}

func (c *Configuration) ensureNoTraefikRebootHooks() error {
	var found []string
	for _, hook := range []string{"pre-traefik-reboot", "post-traefik-reboot"} {
		if c.env.Files.Exists(filepath.Join(c.HooksPath(), hook)) {
			found = append(found, hook)
		}
	}
	if len(found) > 0 {
		return kamalerr.Configuration(fmt.Sprintf("Found %s, these should be renamed to (pre|post)-proxy-reboot", strings.Join(found, ", ")))
	}
	return nil
}

func (c *Configuration) ensureOneHostForSSLRoles() error {
	for _, role := range c.Roles() {
		if err := role.ensureOneHostForSSL(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Configuration) ensureUniqueHostsForSSLRoles() error {
	counts := map[string]int{}
	var order []string
	for _, role := range c.Roles() {
		if !role.SSL() {
			continue
		}
		for _, host := range role.Proxy().Hosts() {
			if counts[host] == 0 {
				order = append(order, host)
			}
			counts[host]++
		}
	}
	return duplicateSSLHostsError(order, counts)
}

func duplicateSSLHostsError(order []string, counts map[string]int) error {
	var duplicates []string
	for _, host := range order {
		if counts[host] > 1 {
			duplicates = append(duplicates, host)
		}
	}
	if len(duplicates) == 0 {
		return nil
	}
	return kamalerr.Configuration("Different roles can't share the same host for SSL: " + strings.Join(duplicates, ", "))
}

func (c *Configuration) ensureLocalRegistryRemoteBuilderHasSSHURL() error {
	if !c.registry.Local() || !c.builder.IsRemote() {
		return nil
	}
	if u, err := url.Parse(c.builder.Remote()); err != nil || u.Scheme != "ssh" {
		return kamalerr.Configuration("Local registry with remote builder requires an SSH URL (e.g., ssh://user@host)")
	}
	return nil
}

func (c *Configuration) ensureNoConflictingProxyRuns() error {
	for _, host := range c.AllHosts() {
		runs := c.proxyRuns(host)
		for _, run := range runs[min(1, len(runs)):] {
			if !run.Equal(runs[0]) {
				return kamalerr.Configuration("Conflicting proxy run configurations for host " + host)
			}
		}
	}
	return nil
}

func (c *Configuration) ensureValidHooksOutput() error {
	setting := c.raw.At("hooks_output")
	if m := setting.Map(); m != nil {
		return c.validateHooksOutputMap(m.Keys(), func(hook string) string { return m.At(hook).ToS() })
	}
	if setting.IsNull() {
		return nil
	}
	return validateHooksOutputLevel(setting.ToS(), "")
}

func (c *Configuration) validateHooksOutputMap(hooks []string, level func(string) string) error {
	for _, hook := range hooks {
		if err := validateHooksOutputLevel(level(hook), hook); err != nil {
			return err
		}
	}
	return nil
}

func validateHooksOutputLevel(level, hook string) error {
	if utils.Contains(hooksOutputLevels, level) {
		return nil
	}
	context := ""
	if hook != "" {
		context = fmt.Sprintf(" for hook '%s'", hook)
	}
	return kamalerr.Configuration(fmt.Sprintf("Invalid hooks_output '%s'%s, must be one of: %s", level, context, strings.Join(hooksOutputLevels, ", ")))
}

// RoleNames returns web for a host list, or the sorted role names, as the private
// role_names of Configuration does for to_h.
func (c *Configuration) RoleNames() []string {
	servers := c.raw.At("servers")
	if servers.Map() == nil {
		return []string{"web"}
	}
	names := servers.Map().Keys()
	sort.Strings(names)
	return names
}
