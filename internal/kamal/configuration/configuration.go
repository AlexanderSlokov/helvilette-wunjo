// Package configuration reads a Kamal deploy file and answers every question that
// the commands ask about it: hosts, roles, images, env, proxy, and builder.
//
// Ports lib/kamal/configuration.rb and lib/kamal/configuration/ at Kamal 2.12.0.
package configuration

import (
	"io"
	"os"
	"path/filepath"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/git"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/secrets"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// FileSystem reads local files: deploy files, accessory files, and hooks.
type FileSystem interface {
	ReadFile(path string) ([]byte, error)
	Exists(path string) bool
}

// OSFileSystem reads files from disk.
type OSFileSystem struct{}

// ReadFile returns the content of path.
func (OSFileSystem) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

// Exists reports whether path exists.
func (OSFileSystem) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Environment holds what the configuration reads outside the deploy file. Tests
// pass fakes for every field.
type Environment struct {
	// Lookup reads process environment variables, such as VERSION.
	Lookup func(name string) (string, bool)
	// Shell runs local commands, such as `docker info` for the local arch.
	Shell localshell.Shell
	// Git reads the local checkout for the default version.
	Git git.Git
	// Files reads accessory files and checks for hooks.
	Files FileSystem
	// SecretFiles reads the .kamal/secrets files.
	SecretFiles secrets.Files
	// Substitution runs the $(command) substitutions of secrets files.
	Substitution secrets.Substitution
	// WorkDir is Dir.pwd, and TempDir is Dir.tmpdir.
	WorkDir string
	TempDir string
	// RandomHex returns 16 random hex characters, for uncommitted versions.
	RandomHex func() string
	// Stderr receives warnings.
	Stderr io.Writer
}

// Configuration is a validated deploy file.
type Configuration struct {
	raw             *yamlord.Ordered
	destination     string
	declaredVersion string
	env             Environment

	secrets     *secrets.Secrets
	servers     *Servers
	registry    *Registry
	accessories []*Accessory
	aliases     []*Alias
	boot        *Boot
	builder     *Builder
	rootEnv     *Env
	logging     *Logging
	output      *Output
	proxy       *Proxy
	proxyBoot   *ProxyBoot
	ssh         *SSH
	sshkit      *Sshkit

	version   *versionResult
	arch      *string
	gitIsUsed *bool
}

// Options are the inputs of New besides the deploy file.
type Options struct {
	Destination string
	Version     string
	Env         Environment
}

// New validates raw, the merged deploy file, and builds the configuration in the
// order Kamal does, so the first invalid section reports first.
//
//	cfg, err := configuration.New(raw, configuration.Options{Destination: "staging", Env: env})
func New(raw *yamlord.Ordered, opts Options) (*Configuration, error) {
	c := &Configuration{raw: raw, destination: opts.Destination, declaredVersion: opts.Version, env: withDefaults(opts.Env)}
	c.proxyBoot = &ProxyBoot{cfg: c}
	v := validator{context: "", allowExtensions: true, symbolKeys: true}
	if err := v.againstExample(yamlord.NewMap(raw), exampleDoc("configuration")); err != nil {
		return nil, err
	}
	c.secrets = secrets.New(secrets.Options{Destination: opts.Destination, SecretsPath: c.SecretsPath(), Files: c.env.SecretFiles, Substitution: c.env.Substitution})
	if err := c.buildSections(); err != nil {
		return nil, err
	}
	return c, c.ensureValid()
}

func withDefaults(env Environment) Environment {
	if env.Lookup == nil {
		env.Lookup = func(string) (string, bool) { return "", false }
	}
	if env.Files == nil {
		env.Files = OSFileSystem{}
	}
	if env.SecretFiles == nil {
		env.SecretFiles = secrets.OSFiles{}
	}
	if env.Stderr == nil {
		env.Stderr = io.Discard
	}
	return env
}

func (c *Configuration) buildSections() error {
	steps := []func() error{
		c.buildServers, c.buildRegistry, c.buildAccessories, c.buildAliases, c.buildBoot,
		c.buildBuilder, c.buildEnv, c.buildLogging, c.buildOutput, c.buildProxy, c.buildSSH, c.buildSshkit,
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return c.resolveRoleProxies()
}

func (c *Configuration) buildServers() (err error) {
	c.servers, err = newServers(c)
	return err
}

func (c *Configuration) buildRegistry() (err error) {
	c.registry, err = newRegistry(c.raw, c.secrets, "registry")
	return err
}

func (c *Configuration) buildAccessories() error {
	for _, name := range c.raw.At("accessories").Map().Keys() {
		accessory, err := newAccessory(name, c)
		if err != nil {
			return err
		}
		c.accessories = append(c.accessories, accessory)
	}
	return nil
}

func (c *Configuration) buildAliases() error {
	for _, name := range c.raw.At("aliases").Map().Keys() {
		alias, err := newAlias(name, c)
		if err != nil {
			return err
		}
		c.aliases = append(c.aliases, alias)
	}
	return nil
}

func (c *Configuration) buildBoot() (err error) {
	c.boot, err = newBoot(c.raw.At("boot"), len(c.AllHosts()))
	return err
}

func (c *Configuration) buildBuilder() (err error) {
	c.builder, err = newBuilder(c, c.raw.At("builder"))
	return err
}

func (c *Configuration) buildEnv() (err error) {
	c.rootEnv, err = newEnv(c.raw.At("env"), c.secrets, "env")
	return err
}

func (c *Configuration) buildLogging() (err error) {
	c.logging, err = newLogging(c.raw.At("logging"), "logging")
	return err
}

func (c *Configuration) buildOutput() (err error) {
	c.output, err = newOutput(c.raw.At("output"))
	return err
}

func (c *Configuration) buildProxy() (err error) {
	c.proxy, err = newProxy(c, c.raw.At("proxy"), "", c.secrets, "proxy")
	return err
}

func (c *Configuration) buildSSH() (err error) {
	c.ssh, err = newSSH(c.raw.At("ssh"), c.secrets, c.env.Stderr)
	return err
}

func (c *Configuration) buildSshkit() (err error) {
	c.sshkit, err = newSshkit(c.raw.At("sshkit"))
	return err
}

func (c *Configuration) resolveRoleProxies() error {
	for _, role := range c.Roles() {
		if err := role.resolveProxy(); err != nil {
			return err
		}
	}
	return nil
}

// Raw returns the merged deploy file.
func (c *Configuration) Raw() *yamlord.Ordered { return c.raw }

// Destination returns the destination, such as staging, or "".
func (c *Configuration) Destination() string { return c.destination }

// Secrets returns the secrets of the deploy.
func (c *Configuration) Secrets() *secrets.Secrets { return c.secrets }

// Service returns the service name.
func (c *Configuration) Service() string { return c.raw.At("service").ToS() }

// Labels returns the root labels, or nil.
func (c *Configuration) Labels() *yamlord.Value { return c.raw.At("labels") }

// Registry returns the image registry.
func (c *Configuration) Registry() *Registry { return c.registry }

// Accessories returns the accessories in file order.
func (c *Configuration) Accessories() []*Accessory { return c.accessories }

// Aliases returns the aliases in file order.
func (c *Configuration) Aliases() []*Alias { return c.aliases }

// Boot returns the boot settings.
func (c *Configuration) Boot() *Boot { return c.boot }

// Builder returns the builder settings.
func (c *Configuration) Builder() *Builder { return c.builder }

// Env returns the root env.
func (c *Configuration) Env() *Env { return c.rootEnv }

// Logging returns the root logging.
func (c *Configuration) Logging() *Logging { return c.logging }

// Output returns the deploy log destinations.
func (c *Configuration) Output() *Output { return c.output }

// Proxy returns the root proxy settings.
func (c *Configuration) Proxy() *Proxy { return c.proxy }

// ProxyBoot returns the kamal-proxy container paths and defaults.
func (c *Configuration) ProxyBoot() *ProxyBoot { return c.proxyBoot }

// SSH returns the SSH settings.
func (c *Configuration) SSH() *SSH { return c.ssh }

// Sshkit returns the connection pool settings.
func (c *Configuration) Sshkit() *Sshkit { return c.sshkit }

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

// Image returns the image name, or the service name for a local registry.
func (c *Configuration) Image() string {
	if image := c.raw.At("image"); image.Present() {
		return image.ToS()
	}
	if c.registry.Local() {
		return c.Service()
	}
	return ""
}

// ProxyRun returns the kamal-proxy run settings for host, or nil.
func (c *Configuration) ProxyRun(host string) *ProxyRun {
	if runs := c.proxyRuns(host); len(runs) > 0 {
		return runs[0]
	}
	return nil
}

func (c *Configuration) proxyRuns(host string) []*ProxyRun {
	var runs []*ProxyRun
	for _, role := range c.HostRoles(host) {
		if p := role.Proxy(); p != nil && p.Run() != nil {
			runs = append(runs, p.Run())
		}
	}
	for _, accessory := range c.HostAccessories(host) {
		if p := accessory.Proxy(); p != nil && p.Run() != nil {
			runs = append(runs, p.Run())
		}
	}
	return runs
}

// Repository returns [server/]image.
func (c *Configuration) Repository() string { return joinNonEmpty("/", c.registry.Server(), c.Image()) }

// AbsoluteImage returns <repository>:<version>.
func (c *Configuration) AbsoluteImage() string { return c.Repository() + ":" + c.Version() }

// LatestImage returns <repository>:<latest tag>.
func (c *Configuration) LatestImage() string { return c.Repository() + ":" + c.LatestTag() }

// LatestTag returns latest, or latest-<destination>.
func (c *Configuration) LatestTag() string { return joinNonEmpty("-", "latest", c.destination) }

// ServiceWithVersion returns <service>-<version>.
func (c *Configuration) ServiceWithVersion() string { return c.Service() + "-" + c.Version() }

// RequireDestination reports whether a destination is required.
func (c *Configuration) RequireDestination() bool { return c.raw.At("require_destination").Truthy() }

// RetainContainers returns how many old containers to keep, default 5.
func (c *Configuration) RetainContainers() int64 { return c.intOr("retain_containers", 5) }

// VolumeArgs returns --volume flags for the root volumes.
func (c *Configuration) VolumeArgs() []string {
	volumes := c.raw.At("volumes")
	if !volumes.Present() {
		return []string{}
	}
	return wordsOf(utils.Argumentize("--volume", utils.KeysOf(volumes), false))
}

// LoggingArgs returns the root log flags.
func (c *Configuration) LoggingArgs() []string { return wordsOf(c.logging.Args()) }

// ReadinessDelay returns readiness_delay, default 7.
func (c *Configuration) ReadinessDelay() int64 { return c.intOr("readiness_delay", 7) }

// DeployTimeout returns deploy_timeout, default 30.
func (c *Configuration) DeployTimeout() *yamlord.Value { return c.valueOr("deploy_timeout", 30) }

// DrainTimeout returns drain_timeout, default 30.
func (c *Configuration) DrainTimeout() *yamlord.Value { return c.valueOr("drain_timeout", 30) }

// StopTimeout returns stop_timeout, or nil.
func (c *Configuration) StopTimeout() *yamlord.Value { return c.raw.At("stop_timeout") }

// MinimumVersion returns minimum_version, or "".
func (c *Configuration) MinimumVersion() string { return c.raw.At("minimum_version").ToS() }

// RunDirectory returns .kamal, the directory Kamal uses on each host.
func (c *Configuration) RunDirectory() string { return ".kamal" }

// AppsDirectory returns .kamal/apps.
func (c *Configuration) AppsDirectory() string { return filepath.Join(c.RunDirectory(), "apps") }

// AppDirectory returns .kamal/apps/<service>[-<destination>].
func (c *Configuration) AppDirectory() string {
	return filepath.Join(c.AppsDirectory(), c.ServiceAndDestination())
}

// EnvDirectory returns the app's env directory on each host.
func (c *Configuration) EnvDirectory() string { return filepath.Join(c.AppDirectory(), "env") }

// AssetsDirectory returns the app's assets directory on each host.
func (c *Configuration) AssetsDirectory() string { return filepath.Join(c.AppDirectory(), "assets") }

// HooksPath returns hooks_path, default .kamal/hooks.
func (c *Configuration) HooksPath() string { return c.stringOr("hooks_path", ".kamal/hooks") }

// SecretsPath returns secrets_path, default .kamal/secrets.
func (c *Configuration) SecretsPath() string { return c.stringOr("secrets_path", ".kamal/secrets") }

// AssetPath returns asset_path, or nil.
func (c *Configuration) AssetPath() *yamlord.Value { return c.raw.At("asset_path") }

// ErrorPagesPath returns error_pages_path, or nil.
func (c *Configuration) ErrorPagesPath() *yamlord.Value { return c.raw.At("error_pages_path") }

// ServiceAndDestination returns <service>[-<destination>].
func (c *Configuration) ServiceAndDestination() string {
	return joinNonEmpty("-", c.Service(), c.destination)
}

// EnvTags returns the tags of the root env.
func (c *Configuration) EnvTags() []*EnvTag {
	var tags []*EnvTag
	c.raw.At("env").Dig("tags").Map().Each(func(name string, config *yamlord.Value) {
		tags = append(tags, &EnvTag{Name: name, config: config, sec: c.secrets})
	})
	return tags
}

// EnvTag returns the named env tag, or nil.
func (c *Configuration) EnvTag(name string) *EnvTag {
	for _, tag := range c.EnvTags() {
		if tag.Name == name {
			return tag
		}
	}
	return nil
}

// HooksOutputFor returns the output level of hook, quiet or verbose, or "".
func (c *Configuration) HooksOutputFor(hook string) string {
	setting := c.raw.At("hooks_output")
	switch setting.Kind() {
	case yamlord.String, yamlord.Symbol:
		return setting.ToS()
	case yamlord.Map:
		return setting.Map().At(hook).ToS()
	default:
		return ""
	}
}

func (c *Configuration) intOr(key string, fallback int64) int64 {
	if value := c.raw.At(key); value.Truthy() {
		return value.Int()
	}
	return fallback
}

func (c *Configuration) valueOr(key string, fallback int64) *yamlord.Value {
	if value := c.raw.At(key); value.Truthy() {
		return value
	}
	return yamlord.NewInt(fallback)
}

func (c *Configuration) stringOr(key, fallback string) string {
	if value := c.raw.At(key); value.Truthy() {
		return value.ToS()
	}
	return fallback
}

func (c *Configuration) lookupEnv(name string) (string, bool) { return c.env.Lookup(name) }

// absolutePath expands a local path against the working directory, as
// File.expand_path does.
func (c *Configuration) absolutePath(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(c.env.WorkDir, path)
}

func (c *Configuration) dockerArch() string {
	if c.arch == nil {
		arch, _ := utils.DockerArch(c.env.Shell)
		c.arch = &arch
	}
	return *c.arch
}

func (c *Configuration) gitUsed() bool {
	if c.gitIsUsed == nil {
		used := c.env.Git.Shell != nil && c.env.Git.Used()
		c.gitIsUsed = &used
	}
	return *c.gitIsUsed
}

func uniq(items []string) []string {
	var out []string
	for _, item := range items {
		if !utils.Contains(out, item) {
			out = append(out, item)
		}
	}
	return out
}
