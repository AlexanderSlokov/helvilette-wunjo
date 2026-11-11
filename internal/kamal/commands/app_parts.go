package commands

// Ports lib/kamal/commands/app/*.rb at Kamal 2.12.0.

import (
	"path/filepath"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

const dockerHealthLogFormat = "'{{json .State.Health}}'"

// ExtractAssets copies the role's asset path out of the new image.
func (a *App) ExtractAssets() Line {
	container := a.role.ContainerPrefix() + "-assets"
	extracted := a.role.AssetExtractedDirectory(a.cfg.Version())
	return combine("&&",
		makeDirectory(extracted),
		cat(docker(w("container", "rm", container, "2> /dev/null")), w("|| true")),
		docker(w("container", "create", "--name", container, a.cfg.AbsoluteImage())),
		docker(w("container", "cp", "-L", container+":"+a.role.AssetPath()+"/.", extracted)),
		docker(w("container", "rm", container)),
	)
}

// SyncAssetVolumes merges the assets of the new and old versions into both volumes,
// so requests for either version find their files during the switch.
func (a *App) SyncAssetVolumes(oldVersion string) Line {
	newExtracted := a.role.AssetExtractedDirectory(a.cfg.Version())
	newVolume, _ := a.role.AssetVolume(a.cfg.Version())
	commands := []Line{makeDirectory(newVolume.HostPath), copyContents(newExtracted, newVolume.HostPath, false)}
	if strings.TrimSpace(oldVersion) != "" {
		oldExtracted := a.role.AssetExtractedDirectory(oldVersion)
		oldVolume, _ := a.role.AssetVolume(oldVersion)
		commands = append(commands, copyContents(newExtracted, oldVolume.HostPath, true), copyContents(oldExtracted, newVolume.HostPath, true))
	}
	return chain(commands...)
}

// CleanUpAssets removes the extracted assets and volumes of older versions.
func (a *App) CleanUpAssets() Line {
	return chain(
		a.findAndRemoveOlderSiblings(a.role.AssetExtractedDirectory(a.cfg.Version())),
		a.findAndRemoveOlderSiblings(a.role.AssetVolumeDirectory(a.cfg.Version())),
	)
}

func (a *App) findAndRemoveOlderSiblings(path string) Line {
	return w("find", filepath.Dir(path), "-maxdepth 1", "-name", "'"+a.role.Name()+"-*'", "!", "-name", filepath.Base(path), `-exec rm -rf "{}" +`)
}

func copyContents(source, destination string, continueOnError bool) Line {
	return cat(w("cp", "-rnT", source, destination), when(continueOnError, "|| true"))
}

// ListContainers returns docker container ls --all of the role.
func (a *App) ListContainers() Line {
	return docker(w("container", "ls", "--all"), a.containerFilterArgs(nil))
}

// ListContainerNames prints the names of the role's containers.
func (a *App) ListContainerNames() Line {
	return cat(a.ListContainers(), w("--format", "'{{ .Names }}'"))
}

// RemoveContainer removes the container of version.
func (a *App) RemoveContainer(version string) Line {
	return pipe(a.ContainerIDFor(a.containerName(version), false), xargs(docker(w("container", "rm"))))
}

// RenameContainer renames the container of version to that of newVersion.
func (a *App) RenameContainer(version, newVersion string) Line {
	return docker(w("rename", a.containerName(version), a.containerName(newVersion)))
}

// RemoveContainers prunes the role's stopped containers.
func (a *App) RemoveContainers() Line {
	return docker(w("container", "prune", "--force"), a.containerFilterArgs(nil))
}

// ContainerHealthLog prints the health check log of the container of version.
func (a *App) ContainerHealthLog(version string) Line {
	return pipe(a.ContainerIDFor(a.containerName(version), false), xargs(docker(w("inspect", "--format", dockerHealthLogFormat))))
}

// CreateErrorPagesDirectory returns mkdir -p of the app's error pages directory.
func (a *App) CreateErrorPagesDirectory() Line {
	return makeDirectory(a.cfg.ProxyBoot().ErrorPagesDirectory())
}

// CleanUpErrorPages removes the error pages of other versions.
func (a *App) CleanUpErrorPages() Line {
	return w("find", a.cfg.ProxyBoot().ErrorPagesDirectory(), "-mindepth", "1", "-maxdepth", "1", "!", "-name", a.cfg.Version(), "-exec", "rm", "-rf", "{} +")
}

// ExecuteInExistingContainer returns docker exec in the current container.
func (a *App) ExecuteInExistingContainer(command []string, interactive bool, env *yamlord.Ordered) Line {
	return docker(w("exec"), when(interactive, a.dockerInteractiveArgs()),
		utils.Argumentize("--env", utils.AttrsOf(env), false), w(a.containerName("")), w(command...))
}

// ExecuteInNewContainer returns docker run of a one-off container with the role's
// env, volumes, and options.
func (a *App) ExecuteInNewContainer(command []string, interactive, detach bool, env *yamlord.Ordered) Line {
	var roleEnv, roleOptions Line
	if a.role != nil {
		roleEnv, roleOptions = a.role.EnvArgs(a.host), a.role.OptionArgs()
	}
	return docker(w("run"), when(interactive, a.dockerInteractiveArgs()), when(detach, "--detach"), when(!detach, "--rm"),
		w("--name", a.containerNameForExec(), "--network", "kamal"), roleEnv,
		utils.Argumentize("--env", utils.AttrsOf(env), false), a.role.LoggingArgs(), w(a.cfg.VolumeArgs()...),
		roleOptions, w(a.cfg.AbsoluteImage()), w(command...))
}

// ExecuteInExistingContainerOverSSH returns the ssh command for an interactive exec.
func (a *App) ExecuteInExistingContainerOverSSH(command []string, env *yamlord.Ordered) string {
	return a.RunOverSSH(a.ExecuteInExistingContainer(command, true, env), a.host)
}

// ExecuteInNewContainerOverSSH returns the ssh command for an interactive run.
func (a *App) ExecuteInNewContainerOverSSH(command []string, env *yamlord.Ordered) string {
	return a.RunOverSSH(a.ExecuteInNewContainer(command, true, false, env), a.host)
}

func (a *App) containerNameForExec() string {
	return strings.Join([]string{a.role.ContainerPrefix(), "exec", a.cfg.Version(), a.rt.RandomHex(3)}, "-")
}

// ListImages returns docker image ls of the repository.
func (a *App) ListImages() Line { return docker(w("image", "ls", a.cfg.Repository())) }

// RemoveImages prunes the service's images.
func (a *App) RemoveImages() Line {
	return docker(w("image", "prune", "--all", "--force"), a.imageFilterArgs())
}

// TagLatestImage tags the current image as latest.
func (a *App) TagLatestImage() Line {
	return docker(w("tag", a.cfg.AbsoluteImage(), a.cfg.LatestImage()))
}

// LogOptions are the options of the logs commands. An empty ContainerID means the
// running container; ContainerCommand, when set, replaces the lookup.
type LogOptions struct {
	ContainerID      string
	ContainerCommand Line
	Timestamps       bool
	Since            string
	Lines            string
	Grep             string
	GrepOptions      string
}

// Logs returns docker logs of a container, optionally filtered with grep.
func (a *App) Logs(opts LogOptions) Line {
	return pipe(a.containerIDCommand(opts), w(dockerLogs(opts, "")), grepFilter(opts, "'"))
}

// FollowLogs returns the ssh command that follows the logs on host.
func (a *App) FollowLogs(host string, opts LogOptions) string {
	return a.RunOverSSH(pipe(a.containerIDCommand(opts), w(dockerLogs(opts, " --follow")), grepFilter(opts, `"`)), host)
}

func dockerLogs(opts LogOptions, follow string) string {
	text := "xargs docker logs"
	if opts.Timestamps {
		text += " --timestamps"
	}
	if opts.Since != "" {
		text += " --since " + opts.Since
	}
	if opts.Lines != "" {
		text += " --tail " + opts.Lines
	}
	return text + follow + " 2>&1"
}

// grepFilter returns grep '<pattern>' [options], or nil without a pattern. Kamal
// quotes the pattern with single quotes locally and double quotes over ssh.
func grepFilter(opts LogOptions, quote string) Line {
	if opts.Grep == "" {
		return nil
	}
	text := "grep " + quote + opts.Grep + quote
	if opts.GrepOptions != "" {
		text += " " + opts.GrepOptions
	}
	return w(text)
}

func (a *App) containerIDCommand(opts LogOptions) Line {
	switch {
	case opts.ContainerCommand != nil:
		return opts.ContainerCommand
	case opts.ContainerID != "":
		return shellCommand(w("echo " + opts.ContainerID))
	default:
		return a.CurrentRunningContainerID()
	}
}

// Deploy registers the target container with kamal-proxy.
func (a *App) Deploy(target string) Line {
	return a.proxyExec(w("deploy", a.role.ContainerPrefix()), a.role.Proxy().DeployCommandArgs(target))
}

// Remove deregisters the role from kamal-proxy.
func (a *App) Remove() Line { return a.proxyExec(w("remove", a.role.ContainerPrefix())) }

// Live resumes traffic to the role.
func (a *App) Live() Line { return a.proxyExec(w("resume", a.role.ContainerPrefix())) }

// Maintenance stops traffic to the role, with an optional drain timeout and message.
func (a *App) Maintenance(drainTimeout *yamlord.Value, message string) Line {
	return a.proxyExec(w("stop", a.role.ContainerPrefix()), a.role.Proxy().StopCommandArgs(drainTimeout, message))
}

// RemoveProxyAppDirectory removes the app's proxy directory.
func (a *App) RemoveProxyAppDirectory() Line {
	return a.RemoveDirectory(a.cfg.ProxyBoot().AppDirectory())
}

// CreateSSLDirectory returns mkdir -p of the role's TLS directory.
func (a *App) CreateSSLDirectory() Line {
	return makeDirectory(filepath.Join(a.cfg.ProxyBoot().TLSDirectory(), a.role.Name()))
}

func (a *App) proxyExec(parts ...Line) Line {
	return docker(append([]Line{w("exec", a.cfg.ProxyBoot().ContainerName(), "kamal-proxy")}, parts...)...)
}
