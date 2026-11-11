package commands

// Ports lib/kamal/commands/builder.rb, builder/base.rb, and builder/clone.rb at
// Kamal 2.12.0.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// BuilderError reports a build that cannot start, such as a missing Dockerfile.
type BuilderError struct{ Message string }

func (e *BuilderError) Error() string { return e.Message }

// BuildDriver is one way to build the image: local, remote, hybrid, pack, or cloud.
type BuildDriver interface {
	Name() string
	Create() (Line, error)
	Remove() (Line, error)
	Info() Line
	InspectBuilder() (Line, error)
	Push(exportAction string, tagAsDirty, noCache bool) (Line, error)
	BuildOptions() (Line, error)
	LoginToRegistryLocally() bool
	PushEnv() []utils.Attr
}

// Builder picks the build driver from the configuration and builds the commands
// shared by every driver.
type Builder struct {
	Base
}

// Target returns the driver: hybrid or remote with remote arches, then pack, cloud,
// or local.
func (b Builder) Target() BuildDriver {
	cfg := b.cfg.Builder()
	base := builderBase{Base: b.Base}
	switch {
	case cfg.IsRemote() && cfg.IsLocal():
		return hybridBuilder{remoteBuilder{base}}
	case cfg.IsRemote():
		return remoteBuilder{base}
	case cfg.Pack():
		return packBuilder{base}
	case cfg.Cloud():
		return cloudBuilder{base}
	default:
		return localBuilder{base}
	}
}

// Name returns the driver name, such as local.
func (b Builder) Name() string { return b.Target().Name() }

// Clean removes the local image.
func (b Builder) Clean() Line { return docker(w("image", "rm", "--force", b.cfg.AbsoluteImage())) }

// Pull pulls the image.
func (b Builder) Pull() Line { return docker(w("pull", b.cfg.AbsoluteImage())) }

// ValidateImage checks that the image carries the service label.
func (b Builder) ValidateImage() Line {
	image := b.cfg.AbsoluteImage()
	return pipe(docker(w("inspect", "-f", "'{{ .Config.Labels.service }}'", image)),
		anyOf(w("grep", "-x", b.cfg.Service()), w(`(echo "Image `+image+` is missing the 'service' label" && exit 1)`)))
}

// FirstMirror prints the first registry mirror of the local Docker.
func (b Builder) FirstMirror() Line {
	return docker(w("info", "--format '{{index .RegistryConfig.Mirrors 0}}'"))
}

// Clone clones the repository into the clone directory.
func (b Builder) Clone() (Line, error) {
	root, err := b.escapedRoot()
	if err != nil {
		return nil, err
	}
	return gitIn(shell.Escape(b.cfg.Builder().CloneDirectory()), "clone", root, "--recurse-submodules"), nil
}

// CloneResetSteps returns the commands that bring an existing clone to HEAD.
func (b Builder) CloneResetSteps() ([]Line, error) {
	root, err := b.escapedRoot()
	if err != nil {
		return nil, err
	}
	revision, err := b.rt.Git.Revision()
	if err != nil {
		return nil, err
	}
	dir, err := b.escapedBuildDirectory()
	return []Line{
		gitIn(dir, "remote", "set-url", "origin", root), gitIn(dir, "fetch", "origin"),
		gitIn(dir, "reset", "--hard", revision), gitIn(dir, "clean", "-fdx"),
		gitIn(dir, "submodule", "update", "--init"), gitIn(dir, "gc", "--auto", "--quiet"),
	}, err
}

// CloneStatus returns git status of the clone.
func (b Builder) CloneStatus() (Line, error) {
	dir, err := b.escapedBuildDirectory()
	return gitIn(dir, "status", "--porcelain"), err
}

// CloneRevision returns git rev-parse HEAD of the clone.
func (b Builder) CloneRevision() (Line, error) {
	dir, err := b.escapedBuildDirectory()
	return gitIn(dir, "rev-parse", "HEAD"), err
}

func (b Builder) escapedRoot() (string, error) {
	root, err := b.rt.Git.Root()
	return shell.Escape(root), err
}

func (b Builder) escapedBuildDirectory() (string, error) {
	dir, err := b.cfg.Builder().BuildDirectory()
	return shell.Escape(dir), err
}

// builderBase holds what every driver shares, as Builder::Base does.
type builderBase struct{ Base }

func (b builderBase) builderConfig() *configuration.Builder { return b.cfg.Builder() }

func (b builderBase) info() Line {
	return combine("&&", docker(w("context", "ls")), docker(w("buildx", "ls")))
}

// push builds with buildx and pushes, as Builder::Base#push does.
func (b builderBase) push(builderName func() (string, error), exportAction string, tagAsDirty, noCache bool) (Line, error) {
	options, err := b.buildOptions()
	if err != nil {
		return nil, err
	}
	builder, err := b.builderFlag(builderName)
	if err != nil {
		return nil, err
	}
	return docker(w("buildx", "build", "--output=type="+exportAction), b.platformOptions(b.builderConfig().Arches()),
		builder, b.buildTagOptions(tagAsDirty), options, when(noCache, "--no-cache"), w(b.builderConfig().Context(), "2>&1")), nil
}

func (b builderBase) builderFlag(builderName func() (string, error)) (Line, error) {
	if b.builderConfig().DockerDriver() {
		return nil, nil
	}
	name, err := builderName()
	return w("--builder", name), err
}

// buildOptions returns cache, label, args, secrets, file, target, ssh, provenance,
// and sbom flags.
func (b builderBase) buildOptions() (Line, error) {
	cfg := b.builderConfig()
	dockerfile, err := b.buildDockerfile()
	if err != nil {
		return nil, err
	}
	return cat(b.buildCache(), utils.Argumentize("--label", []utils.Attr{{Key: "service", Value: yamlord.NewString(b.cfg.Service())}}, false),
		utils.Argumentize("--build-arg", utils.AttrsOf(cfg.Args()), true), b.buildSecrets(), dockerfile,
		when(cfg.Target() != "", "--target", cfg.Target()), when(cfg.SSH() != "", "--ssh", cfg.SSH()),
		valueFlag("--provenance", cfg.Provenance()), valueFlag("--sbom", cfg.Sbom())), nil
}

func valueFlag(flag string, value *yamlord.Value) Line {
	if value.IsNull() {
		return nil
	}
	return w(flag, value.ToS())
}

func (b builderBase) buildCache() Line {
	cfg := b.builderConfig()
	if cfg.CacheTo() == "" || cfg.CacheFrom() == "" {
		return nil
	}
	return w("--cache-to", cfg.CacheTo(), "--cache-from", cfg.CacheFrom())
}

func (b builderBase) buildSecrets() Line {
	var attrs []utils.Attr
	for _, key := range b.builderConfig().SecretKeys() {
		attrs = append(attrs, utils.Attr{Key: "id", Value: yamlord.NewString(key)})
	}
	return utils.Argumentize("--secret", attrs, false)
}

func (b builderBase) buildDockerfile() (Line, error) {
	dockerfile := b.builderConfig().Dockerfile()
	path := dockerfile
	if !filepath.IsAbs(path) {
		path = filepath.Join(b.rt.WorkDir, path)
	}
	if !b.rt.Files.Exists(path) {
		return nil, &BuilderError{Message: "Missing " + dockerfile}
	}
	return w("--file", dockerfile), nil
}

func (b builderBase) buildTagOptions(tagAsDirty bool) Line {
	var line Line
	for _, name := range []string{b.cfg.AbsoluteImage(), b.cfg.LatestImage()} {
		if tagAsDirty {
			name += "-dirty"
		}
		line = append(line, w("-t", name)...)
	}
	return line
}

func (b builderBase) driverOptions() Line {
	return when(b.cfg.Registry().Local(), "--driver-opt", "network=host")
}

func (b builderBase) platformOptions(arches []string) Line {
	if len(arches) == 0 {
		return nil
	}
	platforms := ""
	for i, arch := range arches {
		if i > 0 {
			platforms += ","
		}
		platforms += "linux/" + arch
	}
	return w("--platform", platforms)
}

// registryDigest identifies the registry account. Buildx caches credentials on the
// builder, so each account needs its own builder; the digest names the account
// without exposing it.
func (b builderBase) registryDigest() (string, error) {
	registry := b.cfg.Registry()
	server := registry.Server()
	if server == "" {
		server = "docker.io"
	}
	user, err := registry.Username()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(server + "\x00" + user))
	return hex.EncodeToString(sum[:])[:12], nil
}

func (b builderBase) withDigest(format string) (string, error) {
	digest, err := b.registryDigest()
	return fmt.Sprintf(format, digest), err
}
