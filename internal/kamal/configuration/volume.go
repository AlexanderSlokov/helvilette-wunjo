package configuration

// Ports lib/kamal/configuration/volume.rb at Kamal 2.12.0.

import (
	"path/filepath"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
)

// Volume is a bind mount from a host path to a container path.
type Volume struct {
	HostPath      string
	ContainerPath string
	Options       string
}

// DockerArgs returns --volume host:container[:options].
func (v Volume) DockerArgs() shell.Line {
	return utils.Argumentize("--volume", utils.Keys(v.DockerArgsString()), false)
}

// DockerArgsString returns host:container[:options], with a relative host path
// under $PWD.
func (v Volume) DockerArgsString() string {
	text := v.hostPathForDocker() + ":" + v.ContainerPath
	if v.Options != "" {
		text += ":" + v.Options
	}
	return text
}

func (v Volume) hostPathForDocker() string {
	if filepath.IsAbs(v.HostPath) {
		return v.HostPath
	}
	return "$PWD/" + v.HostPath
}
