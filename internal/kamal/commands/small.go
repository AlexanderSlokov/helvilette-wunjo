package commands

// Ports lib/kamal/commands/docker.rb, server.rb, registry.rb, prune.rb, and hook.rb
// at Kamal 2.12.0.

import (
	"path/filepath"
	"strconv"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/tags"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
)

// Docker builds the commands that install and check Docker on a host.
type Docker struct{ Base }

// Install pipes the get.docker.com script into sh.
func (d Docker) Install() Line {
	return pipe(shellCommand(anyOf(w("curl", "-fsSL", "https://get.docker.com"), w("wget", "-O -", "https://get.docker.com"), w("echo", `"exit 1"`))), w("sh"))
}

// Installed checks the Docker client.
func (d Docker) Installed() Line { return docker(w("-v")) }

// Running checks the Docker server.
func (d Docker) Running() Line { return docker(w("version")) }

// Superuser checks for root or passwordless sudo.
func (d Docker) Superuser() Line {
	return w(`[ "${EUID:-$(id -u)}" -eq 0 ] || sudo -nl usermod >/dev/null`)
}

// Root checks for root.
func (d Docker) Root() Line { return w(`[ "${EUID:-$(id -u)}" -eq 0 ]`) }

// InDockerGroup checks that the user is in the docker group.
func (d Docker) InDockerGroup() Line { return w(`id -nG "${USER:-$(id -un)}" | grep -qw docker`) }

// AddToDockerGroup adds the user to the docker group.
func (d Docker) AddToDockerGroup() Line { return w(`sudo -n usermod -aG docker "${USER:-$(id -un)}"`) }

// RefreshSession ends the session, so the next one has the new group.
func (d Docker) RefreshSession() Line { return w("kill -HUP $PPID") }

// CreateNetwork creates the kamal network.
func (d Docker) CreateNetwork() Line { return docker(w("network", "create", "kamal")) }

// Server builds the commands of the Kamal directories on a host.
type Server struct{ Base }

// EnsureRunDirectory returns mkdir -p .kamal.
func (s Server) EnsureRunDirectory() Line { return makeDirectory(s.cfg.RunDirectory()) }

// RemoveAppDirectory removes the app's directory.
func (s Server) RemoveAppDirectory() Line { return s.RemoveDirectory(s.cfg.AppDirectory()) }

// AppDirectoryCount counts the apps on the host.
func (s Server) AppDirectoryCount() Line { return pipe(w("ls", s.cfg.AppsDirectory()), w("wc", "-l")) }

// Registry builds docker login, logout, and the local registry commands.
type Registry struct{ Base }

// Login returns docker login with sensitive credentials, or nil for a local
// registry. A nil registry means the deploy file's.
func (r Registry) Login(registry *configuration.Registry) (Line, error) {
	registry = r.orDefault(registry)
	if registry.Local() {
		return nil, nil
	}
	user, err := registry.Username()
	if err != nil {
		return nil, err
	}
	password, err := registry.Password()
	if err != nil {
		return nil, err
	}
	return docker(w("login"), when(registry.Server() != "", registry.Server()), w("-u"), sensitiveValue(user), w("-p"), sensitiveValue(password)), nil
}

func sensitiveValue(value string) Line {
	return Line{shell.Sensitive(utils.EscapeShellValue(value), shell.DefaultRedaction)}
}

// Logout returns docker logout of the registry.
func (r Registry) Logout(registry *configuration.Registry) Line {
	server := r.orDefault(registry).Server()
	return docker(w("logout"), when(server != "", server))
}

// Setup starts or runs the local registry container.
func (r Registry) Setup(registry *configuration.Registry) Line {
	port, _ := r.orDefault(registry).LocalPort()
	return combine("||", docker(w("start", "kamal-docker-registry")),
		docker(w("run", "--detach", "-p", "127.0.0.1:"+strconv.Itoa(port)+":5000", "--name", "kamal-docker-registry", "registry:3")))
}

// Remove stops and removes the local registry container.
func (r Registry) Remove() Line {
	return combine("&&", docker(w("stop", "kamal-docker-registry")), docker(w("rm", "kamal-docker-registry")))
}

// Local reports whether the deploy file's registry is local.
func (r Registry) Local() bool { return r.cfg.Registry().Local() }

func (r Registry) orDefault(registry *configuration.Registry) *configuration.Registry {
	if registry == nil {
		return r.cfg.Registry()
	}
	return registry
}

// Prune builds the commands that remove old images and containers.
type Prune struct{ Base }

// DanglingImages prunes untagged images of the service.
func (p Prune) DanglingImages() Line {
	return docker(w("image", "prune", "--force", "--filter", "label=service="+p.cfg.Service()))
}

// TaggedImages removes tagged images that no container uses.
func (p Prune) TaggedImages() Line {
	return pipe(docker(w("image", "ls"), p.serviceFilter(), w("--format", "'{{.ID}} {{.Repository}}:{{.Tag}}'")),
		grep(w(`-v -w "`+p.activeImageList()+`"`)), w("while read image tag; do docker rmi $tag; done"))
}

// AppContainers removes stopped app containers beyond the newest retain.
func (p Prune) AppContainers(retain int64) Line {
	stopped := w("--filter", "status=created", "--filter", "status=exited", "--filter", "status=dead")
	return pipe(docker(w("ps", "-q", "-a"), p.serviceFilter(), w("--filter", "label=destination="+p.cfg.Destination()), stopped),
		w("tail -n +"+strconv.FormatInt(retain+1, 10)), w("while read container_id; do docker rm $container_id; done"))
}

// activeImageList matches the images of every container, untagged images of the
// repository, and any latest tag, so pruning keeps them.
func (p Prune) activeImageList() string {
	return `$(docker container ls -a --format '{{.Image}}\|' --filter label=service=` + p.cfg.Service() + ` | tr -d '\n')` +
		p.cfg.Repository() + `:<none>\|[^ ]*:latest\(-[^ ]*\)\?$`
}

func (p Prune) serviceFilter() Line { return w("--filter", "label=service="+p.cfg.Service()) }

// Hook builds the commands of the local hooks.
type Hook struct{ Base }

// Run returns the path of the hook.
func (h Hook) Run(hook string) Line { return w(h.hookFile(hook)) }

// Env returns the KAMAL_* variables of the hook, and every secret when withSecrets.
func (h Hook) Env(withSecrets bool, details ...tags.Tag) ([]tags.Tag, error) {
	env := h.tags(details...).Env()
	if !withSecrets {
		return env, nil
	}
	all, err := h.cfg.Secrets().All()
	if err != nil {
		return nil, err
	}
	for _, key := range all.Keys() {
		env = tags.Tags(env).With(tags.Tag{Name: key, Value: all.At(key).ToS()})
	}
	return env, nil
}

// HookExists reports whether the hook file exists.
func (h Hook) HookExists(hook string) bool { return h.rt.Files.Exists(h.hookFile(hook)) }

func (h Hook) hookFile(hook string) string { return filepath.Join(h.cfg.HooksPath(), hook) }
