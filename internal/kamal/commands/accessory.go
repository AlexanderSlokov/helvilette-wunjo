package commands

// Ports lib/kamal/commands/accessory.rb and accessory/proxy.rb at Kamal 2.12.0.

import (
	"fmt"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration"
)

// Accessory builds the commands of one accessory.
type Accessory struct {
	Base
	accessory *configuration.Accessory
}

// NewAccessory returns the commands of the named accessory.
//
//	mysql := commands.NewAccessory(base, "mysql")
func NewAccessory(base Base, name string) *Accessory {
	return &Accessory{Base: base, accessory: base.cfg.Accessory(name)}
}

// Run returns docker run of the accessory. A non-empty host adds KAMAL_HOST.
func (a *Accessory) Run(host string) (Line, error) {
	volumes, err := a.accessory.VolumeArgs()
	if err != nil {
		return nil, err
	}
	acc := a.accessory
	return docker(
		w("run", "--name", acc.ServiceName(), "--detach", "--restart", acc.RestartPolicy()),
		acc.NetworkArgs(), w(a.cfg.LoggingArgs()...), acc.PublishArgs(),
		when(host != "", "--env", `KAMAL_HOST="`+host+`"`),
		acc.EnvArgs(), volumes, acc.LabelArgs(), acc.OptionArgs(), w(acc.Image()), w(acc.Cmd().StringList()...),
	), nil
}

// Start returns docker container start of the accessory.
func (a *Accessory) Start() Line { return docker(w("container", "start", a.accessory.ServiceName())) }

// Stop returns docker container stop of the accessory.
func (a *Accessory) Stop() Line { return docker(w("container", "stop", a.accessory.ServiceName())) }

// Info returns docker ps of the accessory.
func (a *Accessory) Info(all, quiet bool) Line {
	return docker(w("ps"), when(all, "-a"), when(quiet, "-q"), a.serviceFilter())
}

// Logs returns docker logs of the accessory, optionally filtered with grep.
func (a *Accessory) Logs(opts LogOptions) Line {
	return pipe(docker(w("logs", a.accessory.ServiceName()),
		when(opts.Since != "", " --since "+opts.Since), when(opts.Lines != "", " --tail "+opts.Lines),
		when(opts.Timestamps, "--timestamps"), w("2>&1")), grepFilter(opts, "'"))
}

// FollowLogs returns the ssh command that follows the accessory's logs on host.
func (a *Accessory) FollowLogs(host string, opts LogOptions) string {
	return a.RunOverSSH(pipe(docker(w("logs", a.accessory.ServiceName()), when(opts.Timestamps, "--timestamps"),
		when(opts.Since != "", "--since "+opts.Since), when(opts.Lines != "", "--tail "+opts.Lines), w("--follow", "2>&1")),
		grepFilter(opts, `"`)), host)
}

// ExecuteInExistingContainer returns docker exec in the accessory container.
func (a *Accessory) ExecuteInExistingContainer(command []string, interactive bool) Line {
	return docker(w("exec"), when(interactive, a.dockerInteractiveArgs()), w(a.accessory.ServiceName()), w(command...))
}

// ExecuteInNewContainer returns docker run of a one-off accessory container.
func (a *Accessory) ExecuteInNewContainer(command []string, interactive bool) (Line, error) {
	volumes, err := a.accessory.VolumeArgs()
	if err != nil {
		return nil, err
	}
	acc := a.accessory
	return docker(w("run"), when(interactive, a.dockerInteractiveArgs()), w("--rm"), acc.NetworkArgs(), acc.EnvArgs(),
		volumes, acc.OptionArgs(), w(acc.Image()), w(command...)), nil
}

// ExecuteInExistingContainerOverSSH returns the ssh command of an interactive exec
// on the accessory's first host.
func (a *Accessory) ExecuteInExistingContainerOverSSH(command []string) string {
	return a.RunOverSSH(a.ExecuteInExistingContainer(command, true), a.firstHost())
}

// ExecuteInNewContainerOverSSH returns the ssh command of an interactive run on the
// accessory's first host.
func (a *Accessory) ExecuteInNewContainerOverSSH(command []string) (string, error) {
	line, err := a.ExecuteInNewContainer(command, true)
	return a.RunOverSSH(line, a.firstHost()), err
}

func (a *Accessory) firstHost() string {
	if hosts := a.accessory.Hosts(); len(hosts) > 0 {
		return hosts[0]
	}
	return ""
}

// EnsureLocalFilePresent fails when a file to upload is missing locally.
func (a *Accessory) EnsureLocalFilePresent(path configuration.AccessoryPath) error {
	if !path.Rendered && !a.rt.Files.Exists(path.Local) {
		return fmt.Errorf("Missing file: %s", path.Local)
	}
	return nil
}

// PullImage returns docker image pull of the accessory image.
func (a *Accessory) PullImage() Line { return docker(w("image", "pull", a.accessory.Image())) }

// RemoveServiceDirectory returns rm -rf of the accessory's directory.
func (a *Accessory) RemoveServiceDirectory() Line { return w("rm", "-rf", a.accessory.ServiceName()) }

// RemoveContainer prunes the accessory's stopped containers.
func (a *Accessory) RemoveContainer() Line {
	return docker(w("container", "prune", "--force"), a.serviceFilter())
}

// RemoveImage removes the accessory image.
func (a *Accessory) RemoveImage() Line {
	return docker(w("image", "rm", "--force", a.accessory.Image()))
}

// CurrentImageID prints the image ID of the running accessory container.
func (a *Accessory) CurrentImageID() Line {
	return docker(w("inspect", a.accessory.ServiceName(), "--format", "'{{.Image}}'"))
}

// RemoveImageID removes an image by ID. Docker refuses while a container uses it,
// so nothing compares the old image with the new one.
func (a *Accessory) RemoveImageID(imageID string) Line { return docker(w("image", "rm", imageID)) }

// EnsureEnvDirectory returns mkdir -p of the accessory's env directory.
func (a *Accessory) EnsureEnvDirectory() Line { return makeDirectory(a.accessory.EnvDirectory()) }

// Deploy registers the accessory with kamal-proxy.
func (a *Accessory) Deploy(target string) Line {
	return a.proxyExec(w("deploy", a.accessory.ServiceName()), a.accessory.Proxy().DeployCommandArgs(target))
}

// Remove deregisters the accessory from kamal-proxy.
func (a *Accessory) Remove() Line { return a.proxyExec(w("remove", a.accessory.ServiceName())) }

func (a *Accessory) proxyExec(parts ...Line) Line {
	return docker(append([]Line{w("exec", a.cfg.ProxyBoot().ContainerName(), "kamal-proxy")}, parts...)...)
}

func (a *Accessory) serviceFilter() Line {
	return w("--filter", "label=service="+a.accessory.ServiceName())
}
