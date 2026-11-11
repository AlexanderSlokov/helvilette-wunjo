package configuration

// Ports the image, path, and timeout settings of lib/kamal/configuration.rb at
// Kamal 2.12.0.

import (
	"path/filepath"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

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
