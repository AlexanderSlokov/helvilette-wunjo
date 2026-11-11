package commands

// Ports lib/kamal/commands/builder/local.rb, remote.rb, hybrid.rb, pack.rb, and
// cloud.rb at Kamal 2.12.0.

import (
	"regexp"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

const endpointDockerHostInspect = "'{{.Endpoints.docker.Host}}'"

type localBuilder struct{ builderBase }

func (b localBuilder) Name() string { return "local" }

func (b localBuilder) builderName() (string, error) {
	kind := "local"
	if b.cfg.Registry().Local() {
		kind = "local-registry"
	}
	return b.withDigest("kamal-" + kind + "-" + b.builderConfig().Driver() + "-%s")
}

func (b localBuilder) Create() (Line, error) {
	if b.builderConfig().DockerDriver() {
		return nil, nil
	}
	name, err := b.builderName()
	return docker(w("buildx", "create", "--name", name, "--driver="+b.builderConfig().Driver()), b.driverOptions()), err
}

func (b localBuilder) Remove() (Line, error) {
	if b.builderConfig().DockerDriver() {
		return nil, nil
	}
	name, err := b.builderName()
	return docker(w("buildx", "rm", name)), err
}

func (b localBuilder) Info() Line { return b.info() }

func (b localBuilder) InspectBuilder() (Line, error) {
	if b.builderConfig().DockerDriver() {
		return nil, nil
	}
	name, err := b.builderName()
	return docker(w("buildx", "inspect", name)), err
}

func (b localBuilder) Push(exportAction string, tagAsDirty, noCache bool) (Line, error) {
	return b.push(b.builderName, exportAction, tagAsDirty, noCache)
}

func (b localBuilder) BuildOptions() (Line, error) { return b.buildOptions() }

func (b localBuilder) LoginToRegistryLocally() bool { return true }

func (b localBuilder) PushEnv() []utils.Attr { return nil }

type remoteBuilder struct{ builderBase }

func (b remoteBuilder) Name() string { return "remote" }

var nonNameChars = regexp.MustCompile(`[^a-z0-9_-]`)

func (b remoteBuilder) nameSuffix() string {
	suffix := nonNameChars.ReplaceAllString(b.builderConfig().Remote(), "-")
	if b.cfg.Registry().Local() {
		suffix += "-local-registry"
	}
	return suffix
}

func (b remoteBuilder) builderName() (string, error) {
	return b.withDigest("kamal-remote-" + b.nameSuffix() + "-%s")
}

func (b remoteBuilder) contextName(builderName string) string { return builderName + "-context" }

func (b remoteBuilder) Create() (Line, error) {
	name, err := b.builderName()
	return chain(b.createRemoteContext(name), docker(w("buildx", "create", "--name", name), b.driverOptions(), w(b.contextName(name)))), err
}

func (b remoteBuilder) createRemoteContext(name string) Line {
	return docker(w("context", "create", b.contextName(name), "--description", "'"+name+" host'", "--docker", "'host="+b.builderConfig().Remote()+"'"))
}

func (b remoteBuilder) Remove() (Line, error) {
	name, err := b.builderName()
	return chain(docker(w("context", "rm", b.contextName(name))), docker(w("buildx", "rm", name))), err
}

func (b remoteBuilder) Info() Line {
	return chain(docker(w("context", "ls")), docker(w("buildx", "ls")))
}

func (b remoteBuilder) InspectBuilder() (Line, error) {
	name, err := b.builderName()
	return b.inspect(name), err
}

func (b remoteBuilder) inspect(name string) Line {
	context := b.contextName(name)
	buildx := pipe(docker(w("buildx", "inspect", name)), grep(w("-q", "Endpoint:.*"+context)))
	remote := pipe(docker(w("context", "inspect", context, "--format", endpointDockerHostInspect)), grep(w("-xq", b.builderConfig().Remote())))
	return combine("||", combine("&&", buildx, remote), w("(echo no compatible builder && exit 1)"))
}

func (b remoteBuilder) Push(exportAction string, tagAsDirty, noCache bool) (Line, error) {
	return b.push(b.builderName, exportAction, tagAsDirty, noCache)
}

func (b remoteBuilder) BuildOptions() (Line, error) { return b.buildOptions() }

func (b remoteBuilder) LoginToRegistryLocally() bool { return false }

// PushEnv stops buildx from sending the local client token, so the remote builder
// uses its own registry login.
func (b remoteBuilder) PushEnv() []utils.Attr {
	return []utils.Attr{{Key: "BUILDKIT_NO_CLIENT_TOKEN", Value: yamlord.NewString("1")}}
}

type hybridBuilder struct{ remoteBuilder }

func (b hybridBuilder) Name() string { return "hybrid" }

func (b hybridBuilder) builderName() (string, error) {
	return b.withDigest("kamal-hybrid-" + b.builderConfig().Driver() + "-" + b.nameSuffix() + "-%s")
}

func (b hybridBuilder) Create() (Line, error) {
	name, err := b.builderName()
	cfg := b.builderConfig()
	local := docker(w("buildx", "create"), b.platformOptions(cfg.LocalArches()), w("--name", name, "--driver="+cfg.Driver()), b.driverOptions())
	remote := docker(w("buildx", "create"), b.platformOptions(cfg.RemoteArches()), w("--append", "--name", name), b.driverOptions(), w(b.contextName(name)))
	return combine("&&", local, b.createRemoteContext(name), remote), err
}

func (b hybridBuilder) Remove() (Line, error) {
	name, err := b.builderName()
	return chain(docker(w("context", "rm", b.contextName(name))), docker(w("buildx", "rm", name))), err
}

func (b hybridBuilder) InspectBuilder() (Line, error) {
	name, err := b.builderName()
	return b.inspect(name), err
}

func (b hybridBuilder) Push(exportAction string, tagAsDirty, noCache bool) (Line, error) {
	return b.push(b.builderName, exportAction, tagAsDirty, noCache)
}

type packBuilder struct{ builderBase }

func (b packBuilder) Name() string { return "pack" }

func (b packBuilder) Create() (Line, error) { return nil, nil }

func (b packBuilder) Remove() (Line, error) { return nil, nil }

func (b packBuilder) Info() Line {
	return pack(w("builder", "inspect", b.builderConfig().PackBuilder()))
}

func (b packBuilder) InspectBuilder() (Line, error) { return b.Info(), nil }

func (b packBuilder) BuildOptions() (Line, error) { return b.buildOptions() }

func (b packBuilder) LoginToRegistryLocally() bool { return true }

func (b packBuilder) PushEnv() []utils.Attr { return nil }

// Push builds with pack and pushes both tags with docker.
func (b packBuilder) Push(exportAction string, tagAsDirty, noCache bool) (Line, error) {
	build, err := b.build(tagAsDirty, noCache)
	if err != nil {
		return nil, err
	}
	var export Line
	if exportAction == "registry" {
		export = combine("&&", docker(w("push", b.cfg.AbsoluteImage())), docker(w("push", b.cfg.LatestImage())))
	}
	return combine("&&", build, export), nil
}

func (b packBuilder) build(tagAsDirty, noCache bool) (Line, error) {
	cfg := b.builderConfig()
	secrets, err := cfg.Secrets()
	if err != nil {
		return nil, err
	}
	return pack(w("build", b.cfg.Repository(), "--platform", "linux/"+cfg.LocalArches()[0], "--creation-time", "now", "--builder", cfg.PackBuilder()),
		b.buildpacks(), b.buildTagOptions(tagAsDirty), when(noCache, "--clear-cache"), w("--env", "BP_IMAGE_LABELS=service="+b.cfg.Service()),
		utils.Argumentize("--env", utils.AttrsOf(cfg.Args()), false), utils.Argumentize("--env", utils.AttrsOf(secrets), true),
		w("--path", cfg.Context())), nil
}

func (b packBuilder) buildpacks() Line {
	var line Line
	for _, buildpack := range append(b.builderConfig().PackBuildpacks(), "paketo-buildpacks/image-labels") {
		line = append(line, w("--buildpack", buildpack)...)
	}
	return line
}

type cloudBuilder struct{ builderBase }

func (b cloudBuilder) Name() string { return "cloud" }

// builderName turns "cloud org/name" into cloud-org-name.
func (b cloudBuilder) builderName() (string, error) {
	return strings.NewReplacer(" ", "-", "/", "-").Replace(b.builderConfig().Driver()), nil
}

func (b cloudBuilder) Create() (Line, error) {
	return docker(w("buildx", "create", "--driver", b.builderConfig().Driver())), nil
}

func (b cloudBuilder) Remove() (Line, error) {
	name, _ := b.builderName()
	return docker(w("buildx", "rm", name)), nil
}

func (b cloudBuilder) Info() Line { return b.info() }

func (b cloudBuilder) InspectBuilder() (Line, error) {
	name, _ := b.builderName()
	return docker(w("buildx", "inspect", name)), nil
}

func (b cloudBuilder) Push(exportAction string, tagAsDirty, noCache bool) (Line, error) {
	return b.push(b.builderName, exportAction, tagAsDirty, noCache)
}

func (b cloudBuilder) BuildOptions() (Line, error) { return b.buildOptions() }

func (b cloudBuilder) LoginToRegistryLocally() bool { return true }

func (b cloudBuilder) PushEnv() []utils.Attr { return nil }
