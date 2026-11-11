// Package configuration reads a Kamal deploy file and answers every question that
// the commands ask about it: hosts, roles, images, env, proxy, and builder.
//
// Ports lib/kamal/configuration.rb and lib/kamal/configuration/ at Kamal 2.12.0.
package configuration

import (
	"io"
	"sync"
	"os"

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

	// lazyMu guards the values computed on first use, which parallel host
	// sessions read at once.
	lazyMu    sync.Mutex
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

func uniq(items []string) []string {
	var out []string
	for _, item := range items {
		if !utils.Contains(out, item) {
			out = append(out, item)
		}
	}
	return out
}
