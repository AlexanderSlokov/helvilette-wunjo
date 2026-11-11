package commander

import (
	"fmt"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/commands"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/git"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/sshexec"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/tags"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
)

// ConfigOptions are the inputs of the configuration: the deploy file, the
// destination, and the version.
type ConfigOptions struct {
	ConfigFile  string
	Destination string
	Version     string
}

// Commander is the state of one run. It loads the configuration on first use.
type Commander struct {
	env     configuration.Environment
	runtime commands.Runtime
	options *ConfigOptions
	cfg     *configuration.Configuration

	Verbosity        sshexec.Level
	HoldingLock      bool
	Connected        bool
	LockWait         bool
	LockWaitTimeout  int
	LockWaitInterval int

	envLock bool

	specificRoles []*configuration.Role
	specificHosts []string
	specifics     *Specifics
}

// New returns a commander that reads files and runs local commands through env
// and builds commands with runtime. holdingLock is $KAMAL_LOCK == "true", set when
// a parent kamal already holds the lock.
func New(env configuration.Environment, runtime commands.Runtime, holdingLock bool) *Commander {
	return &Commander{env: env, runtime: runtime, Verbosity: sshexec.Info, HoldingLock: holdingLock, envLock: holdingLock,
		LockWaitTimeout: 900, LockWaitInterval: 15}
}

// Configure sets the deploy file to load and drops any loaded configuration.
func (c *Commander) Configure(opts ConfigOptions) {
	c.options, c.cfg, c.specifics = &opts, nil, nil
}

// Configured reports whether Configure was called.
func (c *Commander) Configured() bool { return c.options != nil || c.cfg != nil }

// Config returns the configuration, loading it on first use.
func (c *Commander) Config() (*configuration.Configuration, error) {
	if c.cfg != nil {
		return c.cfg, nil
	}
	if c.options == nil {
		return nil, fmt.Errorf("commander: Configure was not called")
	}
	cfg, err := configuration.CreateFrom(c.options.ConfigFile, configuration.Options{
		Destination: c.options.Destination, Version: c.options.Version, Env: c.env,
	})
	if err != nil {
		return nil, err
	}
	c.cfg = cfg
	return cfg, nil
}

// LoadError wraps a configuration that failed to load inside MustConfig. Kamal
// raises it from wherever the configuration is first used, so MustConfig panics with
// it and the command-line layer recovers it.
type LoadError struct{ Err error }

func (e *LoadError) Error() string { return e.Err.Error() }

func (e *LoadError) Unwrap() error { return e.Err }

// MustConfig returns the configuration, loading it on first use, and panics with a
// *LoadError when it does not load.
func (c *Commander) MustConfig() *configuration.Configuration {
	cfg, err := c.Config()
	if err != nil {
		panic(&LoadError{Err: err})
	}
	return cfg
}

// Reset forgets the configuration and the selection, as KAMAL.reset does before an
// alias runs its command.
func (c *Commander) Reset() {
	*c = *New(c.env, c.runtime, c.envLock)
}

// SetSpecificRoles selects the roles that match the patterns, such as "web*".
func (c *Commander) SetSpecificRoles(patterns []string) error {
	c.specifics = nil
	if len(patterns) == 0 {
		c.specificRoles = nil
		return nil
	}
	roles := c.MustConfig().Roles()
	var names []string
	for _, role := range roles {
		names = append(names, role.Name())
	}
	matched := utils.FilterSpecificItems(patterns, names)
	if len(matched) == 0 {
		return fmt.Errorf("No --roles match for %s", strings.Join(patterns, ","))
	}
	c.specificRoles = nil
	for _, name := range matched {
		c.specificRoles = append(c.specificRoles, c.MustConfig().Role(name))
	}
	return nil
}

// SetSpecificHosts selects the hosts that match the patterns, such as "1.1.1.*".
func (c *Commander) SetSpecificHosts(patterns []string) error {
	c.specifics = nil
	if len(patterns) == 0 {
		c.specificHosts = nil
		return nil
	}
	matched := utils.FilterSpecificItems(patterns, c.MustConfig().AllHosts())
	if len(matched) == 0 {
		return fmt.Errorf("No --hosts match for %s", strings.Join(patterns, ","))
	}
	c.specificHosts = matched
	return nil
}

// SpecificHosts returns the selected hosts, or nil when none were selected.
func (c *Commander) SpecificHosts() []string { return c.specificHosts }

// SpecificRoles returns the selected roles, or nil when none were selected.
func (c *Commander) SpecificRoles() []*configuration.Role { return c.specificRoles }

// SpecificPrimary selects only the primary host of the selected roles, or of the
// configuration.
func (c *Commander) SpecificPrimary() {
	c.specifics = nil
	if len(c.specificRoles) > 0 {
		c.specificHosts = []string{c.specificRoles[0].PrimaryHost()}
		return
	}
	c.specificHosts = []string{c.MustConfig().PrimaryHost()}
}

// WithSpecificHosts runs fn with only hosts selected and restores the selection.
func (c *Commander) WithSpecificHosts(hosts []string, fn func() error) error {
	original := c.specificHosts
	defer func() { c.specificHosts, c.specifics = original, nil }()
	if err := c.SetSpecificHosts(hosts); err != nil {
		return err
	}
	return fn()
}

func (c *Commander) specs() *Specifics {
	if c.specifics == nil {
		c.specifics = newSpecifics(c.MustConfig(), c.specificHosts, c.specificRoles)
	}
	return c.specifics
}

// Hosts returns the selected hosts.
func (c *Commander) Hosts() []string { return c.specs().Hosts() }

// Roles returns the selected roles.
func (c *Commander) Roles() []*configuration.Role { return c.specs().Roles() }

// PrimaryHost returns the primary selected host.
func (c *Commander) PrimaryHost() string { return c.specs().PrimaryHost() }

// PrimaryRole returns the primary selected role.
func (c *Commander) PrimaryRole() *configuration.Role { return c.specs().PrimaryRole() }

// RolesOn returns the selected roles on host.
func (c *Commander) RolesOn(host string) []*configuration.Role { return c.specs().RolesOn(host) }

// AppHosts returns the selected app hosts.
func (c *Commander) AppHosts() []string { return c.specs().AppHosts() }

// ProxyHosts returns the selected proxy hosts.
func (c *Commander) ProxyHosts() []string { return c.specs().ProxyHosts() }

// AccessoryHosts returns the selected accessory hosts.
func (c *Commander) AccessoryHosts() []string { return c.specs().AccessoryHosts() }

// AccessoryNames returns the names of the accessories.
func (c *Commander) AccessoryNames() []string {
	var names []string
	for _, accessory := range c.MustConfig().Accessories() {
		names = append(names, accessory.Name())
	}
	return names
}

func (c *Commander) base() commands.Base { return commands.NewBase(c.MustConfig(), c.runtime) }

// App returns the app commands of role on host.
func (c *Commander) App(role *configuration.Role, host string) *commands.App {
	return commands.NewApp(c.base(), role, host)
}

// Accessory returns the commands of the named accessory.
func (c *Commander) Accessory(name string) *commands.Accessory {
	return commands.NewAccessory(c.base(), name)
}

// Auditor returns the audit commands with extra details.
func (c *Commander) Auditor(details ...tags.Tag) *commands.Auditor {
	return commands.NewAuditor(c.base(), details...)
}

// Builder returns the build commands.
func (c *Commander) Builder() commands.Builder { return commands.Builder{Base: c.base()} }

// Docker returns the Docker setup commands.
func (c *Commander) Docker() commands.Docker { return commands.Docker{Base: c.base()} }

// Hook returns the hook commands.
func (c *Commander) Hook() commands.Hook { return commands.Hook{Base: c.base()} }

// Lock returns the lock commands.
func (c *Commander) Lock() commands.Lock { return commands.Lock{Base: c.base()} }

// Proxy returns the kamal-proxy commands for host.
func (c *Commander) Proxy(host string) *commands.Proxy { return commands.NewProxy(c.base(), host) }

// Prune returns the prune commands.
func (c *Commander) Prune() commands.Prune { return commands.Prune{Base: c.base()} }

// Registry returns the registry commands.
func (c *Commander) Registry() commands.Registry { return commands.Registry{Base: c.base()} }

// Server returns the server commands.
func (c *Commander) Server() commands.Server { return commands.Server{Base: c.base()} }

// ResolveAlias returns the command of the named alias, reading only the raw deploy
// file when the configuration is not loaded yet.
func (c *Commander) ResolveAlias(name string) (string, bool, error) {
	if c.cfg != nil {
		for _, alias := range c.cfg.Aliases() {
			if alias.Name == name {
				return alias.Command, true, nil
			}
		}
		return "", false, nil
	}
	return c.resolveRawAlias(name)
}

func (c *Commander) resolveRawAlias(name string) (string, bool, error) {
	if c.options == nil {
		return "", false, nil
	}
	raw, err := configuration.LoadRawConfig(c.options.ConfigFile, c.options.Destination, c.env)
	if err != nil {
		return "", false, err
	}
	command, ok := raw.At("aliases").Map().Get(name)
	return command.ToS(), ok, nil
}

// Git returns the local Git checkout reader.
func (c *Commander) Git() git.Git { return c.runtime.Git }

// WithVerbosity runs fn at level and restores the previous level.
func (c *Commander) WithVerbosity(level sshexec.Level, fn func() error) error {
	previous := c.Verbosity
	c.Verbosity = level
	defer func() { c.Verbosity = previous }()
	return fn()
}
