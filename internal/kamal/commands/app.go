package commands

// Ports lib/kamal/commands/app.rb at Kamal 2.12.0.

import (
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
)

var activeDockerStatuses = []string{"running", "restarting"}

// App builds the commands for one role of the app on one host.
type App struct {
	Base
	role *configuration.Role
	host string
}

// NewApp returns the app commands of role on host. Both may be empty for commands
// that need neither.
//
//	app := commands.NewApp(base, cfg.Role("web"), "1.1.1.1")
func NewApp(base Base, role *configuration.Role, host string) *App {
	return &App{Base: base, role: role, host: host}
}

func (a *App) containerName(version string) string { return a.role.ContainerName(version) }

// Run returns docker run for a new container of the current version.
func (a *App) Run(hostname string) Line {
	name := a.containerName("")
	return docker(
		w("run", "--detach", "--restart", a.role.RestartPolicy(), "--name", name, "--network", "kamal"),
		when(hostname != "", "--hostname", hostname),
		w("--env", `KAMAL_CONTAINER_NAME="`+name+`"`, "--env", `KAMAL_VERSION="`+a.cfg.Version()+`"`, "--env", `KAMAL_HOST="`+a.host+`"`),
		when(a.cfg.Destination() != "", "--env", `KAMAL_DESTINATION="`+a.cfg.Destination()+`"`),
		a.role.EnvArgs(a.host), a.role.LoggingArgs(), w(a.cfg.VolumeArgs()...), a.role.AssetVolumeArgs(),
		a.role.LabelArgs(), a.role.OptionArgs(), w(a.cfg.AbsoluteImage()), w(a.role.Cmd().StringList()...),
	)
}

// Start returns docker start of the current container.
func (a *App) Start() Line { return docker(w("start", a.containerName(""))) }

// Status returns the health status of the container of version.
func (a *App) Status(version string) Line {
	return pipe(a.ContainerIDForVersion(version, false), xargs(docker(w("inspect", "--format", dockerHealthStatusFormat))))
}

// Stop stops the container of version, or the running one when version is empty.
func (a *App) Stop(version string) Line {
	id := a.CurrentRunningContainerID()
	if version != "" {
		id = a.ContainerIDForVersion(version, false)
	}
	return pipe(id, xargs(docker(w("stop"), a.role.StopArgs())))
}

// Info returns docker ps of the role's containers.
func (a *App) Info() Line { return docker(w("ps"), a.containerFilterArgs(nil)) }

// CurrentRunningContainerID returns the ID of the running container.
func (a *App) CurrentRunningContainerID() Line { return a.currentRunningContainer(w("--quiet")) }

// ContainerIDForVersion returns the ID of the container of version.
func (a *App) ContainerIDForVersion(version string, onlyRunning bool) Line {
	return a.ContainerIDFor(a.containerName(version), onlyRunning)
}

// CurrentRunningVersion prints the version of the running container.
func (a *App) CurrentRunningVersion() Line {
	return pipe(a.currentRunningContainer(w("--format", "'{{.Names}}'")), w(a.extractVersionFromName()))
}

// ListVersions prints the versions of the role's containers, newest first.
func (a *App) ListVersions(dockerArgs []string, statuses []string) Line {
	return pipe(
		docker(w("ps"), a.containerFilterArgs(statuses), w(dockerArgs...), w("--format", `"{{.Names}}"`)),
		w(a.extractVersionFromName()),
	)
}

// EnsureEnvDirectory returns mkdir -p of the role's env directory.
func (a *App) EnsureEnvDirectory() Line { return makeDirectory(a.role.EnvDirectory()) }

func (a *App) latestImageID() Line {
	return docker(w("image", "ls"), utils.Argumentize("--filter", utils.Keys("reference="+a.cfg.LatestImage()), false), w("--format", "'{{.ID}}'"))
}

func (a *App) currentRunningContainer(format Line) Line {
	return pipe(shellCommand(chain(a.latestImageContainer(format), a.latestContainer(format, nil))), w("head", "-1"))
}

func (a *App) latestImageContainer(format Line) Line {
	return a.latestContainer(format, []string{"ancestor=$(" + a.latestImageID().String() + ")"})
}

func (a *App) latestContainer(format Line, filters []string) Line {
	return docker(w("ps", "--latest"), format, a.containerFilterArgs(activeDockerStatuses), utils.Argumentize("--filter", utils.Keys(filters...), false))
}

func (a *App) containerFilterArgs(statuses []string) Line {
	return utils.Argumentize("--filter", utils.Keys(a.containerFilters(statuses)...), false)
}

func (a *App) imageFilterArgs() Line {
	return utils.Argumentize("--filter", utils.Keys("label=service="+a.cfg.Service()), false)
}

// extractVersionFromName strips "<service>-<role>[-<destination>]-" from each name.
func (a *App) extractVersionFromName() string {
	return "while read line; do echo ${line#" + a.role.ContainerPrefix() + "-}; done"
}

func (a *App) containerFilters(statuses []string) []string {
	filters := []string{"label=service=" + a.cfg.Service(), "label=destination=" + a.cfg.Destination()}
	if a.role != nil {
		filters = append(filters, "label=role="+a.role.Name())
	}
	for _, status := range statuses {
		filters = append(filters, "status="+status)
	}
	return filters
}
