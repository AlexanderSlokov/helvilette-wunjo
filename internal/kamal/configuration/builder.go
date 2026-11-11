package configuration

// Ports lib/kamal/configuration/builder.rb at Kamal 2.12.0.

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// Builder holds how Kamal builds the app image: arches, remote builder, cache, and
// build inputs.
type Builder struct {
	cfg    *Configuration
	config *yamlord.Ordered
	once   sync.Once
	arches *archSplit
}

// archSplit memoizes local_arches, which asks the local Docker for its arch.
type archSplit struct {
	local  []string
	remote []string
}

func newBuilder(cfg *Configuration, config *yamlord.Value) (*Builder, error) {
	if config.IsNull() {
		config = yamlord.NewMap(nil)
	}
	if err := validateBuilder(config, example("builder"), "builder"); err != nil {
		return nil, err
	}
	return &Builder{cfg: cfg, config: config.Map()}, nil
}

// ToH returns the raw builder settings.
func (b *Builder) ToH() *yamlord.Ordered { return b.config }

// Remote returns the remote builder URL, or "".
func (b *Builder) Remote() string { return b.config.At("remote").ToS() }

// Arches returns the target architectures, default amd64 and arm64, or none for the
// docker driver.
func (b *Builder) Arches() []string {
	if arch, ok := b.config.Get("arch"); ok {
		return arch.StringList()
	}
	if b.DockerDriver() {
		return nil
	}
	return []string{"amd64", "arm64"}
}

// LocalArches returns the arches that build on this machine: all of them without a
// remote, and only the local Docker's arch with one.
func (b *Builder) LocalArches() []string { return b.split().local }

// RemoteArches returns the arches that build on the remote builder.
func (b *Builder) RemoteArches() []string { return b.split().remote }

func (b *Builder) split() *archSplit {
	b.once.Do(func() {
		local := b.computeLocalArches()
		b.arches = &archSplit{local: local, remote: b.computeRemoteArches(local)}
	})
	return b.arches
}

func (b *Builder) computeLocalArches() []string {
	switch {
	case b.LocalDisabled():
		return []string{}
	case b.Remote() != "":
		return intersect(b.Arches(), []string{b.cfg.dockerArch()})
	default:
		return b.Arches()
	}
}

func (b *Builder) computeRemoteArches(local []string) []string {
	if b.Remote() == "" {
		return []string{}
	}
	return subtract(b.Arches(), local)
}

// IsRemote reports whether any arch builds remotely, as remote? does.
func (b *Builder) IsRemote() bool { return len(b.RemoteArches()) > 0 }

// IsLocal reports whether any arch builds locally, as local? does.
func (b *Builder) IsLocal() bool {
	return !b.LocalDisabled() && (len(b.Arches()) == 0 || len(b.LocalArches()) > 0)
}

// Cloud reports whether the driver is a Docker Build Cloud driver.
func (b *Builder) Cloud() bool { return strings.HasPrefix(b.Driver(), "cloud") }

// Cached reports whether a build cache is set.
func (b *Builder) Cached() bool { return b.config.At("cache").Truthy() }

// Pack reports whether the build uses Cloud Native Buildpacks.
func (b *Builder) Pack() bool { return b.config.At("pack").Truthy() }

// Args returns the build args.
func (b *Builder) Args() *yamlord.Ordered {
	if args := b.config.At("args").Map(); args != nil {
		return args
	}
	return yamlord.NewOrdered()
}

// Secrets returns the build secrets with their values.
func (b *Builder) Secrets() (*yamlord.Ordered, error) {
	out := yamlord.NewOrdered()
	for _, key := range b.config.At("secrets").StringList() {
		value, err := b.cfg.Secrets().Fetch(key)
		if err != nil {
			return nil, err
		}
		out.Set(key, yamlord.NewString(value))
	}
	return out, nil
}

// SecretKeys returns the names of the build secrets.
func (b *Builder) SecretKeys() []string { return b.config.At("secrets").StringList() }

// Dockerfile returns the Dockerfile path, default Dockerfile.
func (b *Builder) Dockerfile() string { return b.stringOr("dockerfile", "Dockerfile") }

// Target returns the build target, or "".
func (b *Builder) Target() string { return b.config.At("target").ToS() }

// Context returns the build context, default ".".
func (b *Builder) Context() string { return b.stringOr("context", ".") }

// Driver returns the buildx driver, default docker-container.
func (b *Builder) Driver() string {
	return b.config.Fetch("driver", yamlord.NewString("docker-container")).ToS()
}

// PackBuilder returns the buildpacks builder image, or "".
func (b *Builder) PackBuilder() string { return b.config.At("pack").Dig("builder").ToS() }

// PackBuildpacks returns the buildpacks.
func (b *Builder) PackBuildpacks() []string {
	return b.config.At("pack").Dig("buildpacks").StringList()
}

// LocalDisabled reports whether local is false.
func (b *Builder) LocalDisabled() bool { return b.config.At("local").IsFalse() }

// CacheFrom returns the --cache-from value, or "" without a cache.
func (b *Builder) CacheFrom() string {
	switch b.cacheType() {
	case "gha":
		return strings.Join(append([]string{"type=gha"}, b.allowedGHAOptions()...), ",")
	case "registry":
		return "type=registry,ref=" + b.cacheImageRef()
	default:
		return ""
	}
}

// CacheTo returns the --cache-to value, or "" without a cache.
func (b *Builder) CacheTo() string {
	switch b.cacheType() {
	case "gha":
		return joinNonEmpty(",", "type=gha", b.cacheOptions())
	case "registry":
		return joinNonEmpty(",", "type=registry", "ref="+b.cacheImageRef(), b.cacheOptions())
	default:
		return ""
	}
}

func (b *Builder) cacheType() string {
	if !b.Cached() {
		return ""
	}
	return b.config.At("cache").Dig("type").ToS()
}

var ghaCacheFromOption = regexp.MustCompile(`^(url|url_v2|token|scope|timeout)=`)

func (b *Builder) allowedGHAOptions() []string {
	var allowed []string
	for _, option := range strings.Split(b.cacheOptions(), ",") {
		if ghaCacheFromOption.MatchString(option) {
			allowed = append(allowed, option)
		}
	}
	return allowed
}

func (b *Builder) cacheOptions() string { return b.config.At("cache").Dig("options").ToS() }

func (b *Builder) cacheImage() string {
	if image := b.config.At("cache").Dig("image"); image.Truthy() {
		return image.ToS()
	}
	return b.cfg.Image() + "-build-cache"
}

func (b *Builder) cacheImageRef() string {
	return joinNonEmpty("/", b.cfg.Registry().Server(), b.cacheImage())
}

// SSH returns the --ssh value, or "".
func (b *Builder) SSH() string { return b.config.At("ssh").ToS() }

// Provenance returns the provenance setting, or nil.
func (b *Builder) Provenance() *yamlord.Value { return b.config.At("provenance") }

// Sbom returns the sbom setting, or nil.
func (b *Builder) Sbom() *yamlord.Value { return b.config.At("sbom") }

// GitClone reports whether Kamal builds from a clean clone of the repository: inside
// Git and without a context setting.
func (b *Builder) GitClone() bool { return b.cfg.gitUsed() && b.config.At("context").IsNull() }

// CloneDirectory returns <tmp>/kamal-clones/<service>-<sha of the working directory>.
func (b *Builder) CloneDirectory() string {
	return filepath.Join(b.cfg.env.TempDir, "kamal-clones", joinNonEmpty("-", b.cfg.Service(), b.pwdSHA()))
}

// BuildDirectory returns where the build runs: inside the clone, or ".".
func (b *Builder) BuildDirectory() (string, error) {
	if !b.GitClone() {
		return ".", nil
	}
	root, err := b.cfg.env.Git.Root()
	if err != nil {
		return "", err
	}
	relative := strings.TrimPrefix(b.cfg.env.WorkDir, root)
	return rubyFileJoin(b.CloneDirectory(), filepath.Base(root), relative), nil
}

// DockerDriver reports whether the driver is docker.
func (b *Builder) DockerDriver() bool { return b.Driver() == "docker" }

func (b *Builder) pwdSHA() string {
	sum := sha256.Sum256([]byte(b.cfg.env.WorkDir))
	return hex.EncodeToString(sum[:])[:13]
}

func (b *Builder) stringOr(key, fallback string) string {
	if value := b.config.At(key); value.Truthy() {
		return value.ToS()
	}
	return fallback
}

// rubyFileJoin joins parts with one slash between them, as Ruby's File.join does. An
// empty last part leaves a trailing slash.
func rubyFileJoin(parts ...string) string {
	out := ""
	for i, part := range parts {
		if i == 0 {
			out = part
			continue
		}
		out = strings.TrimRight(out, "/") + "/" + strings.TrimLeft(part, "/")
	}
	return out
}

func intersect(a, b []string) []string {
	out := []string{}
	for _, item := range a {
		if utils.Contains(b, item) && !utils.Contains(out, item) {
			out = append(out, item)
		}
	}
	return out
}

func subtract(a, b []string) []string {
	out := []string{}
	for _, item := range a {
		if !utils.Contains(b, item) {
			out = append(out, item)
		}
	}
	return out
}
