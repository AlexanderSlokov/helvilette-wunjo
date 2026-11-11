package commands

// Ports lib/kamal/commands/proxy.rb at Kamal 2.12.0.

import (
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration"
)

const proxyConfigVolume = "kamal-proxy-config:/home/kamal-proxy/.config/kamal-proxy"

// Proxy builds the commands of the kamal-proxy container on one host.
type Proxy struct {
	Base
	run *configuration.ProxyRun
}

// NewProxy returns the proxy commands for host, with the host's run settings.
func NewProxy(base Base, host string) *Proxy { return &Proxy{Base: base, run: base.cfg.ProxyRun(host)} }

func (p *Proxy) containerName() string { return p.cfg.ProxyBoot().ContainerName() }

// Run starts kamal-proxy: from the run settings when the deploy file has them, or
// from the boot files on the host.
func (p *Proxy) Run() Line {
	if p.run == nil {
		return pipe(p.BootConfig(), xargs(p.dockerRun()))
	}
	return docker(w("run", "--name", p.containerName(), "--network", "kamal", "--detach", "--restart", "unless-stopped", "--volume", proxyConfigVolume),
		w(p.run.DockerOptionsArgs()...), w(p.run.Image()), w(p.run.RunCommand()))
}

// Start returns docker container start of kamal-proxy.
func (p *Proxy) Start() Line { return docker(w("container", "start", p.containerName())) }

// Stop returns docker container stop of name, default kamal-proxy.
func (p *Proxy) Stop(name string) Line {
	if name == "" {
		name = p.containerName()
	}
	return docker(w("container", "stop", name))
}

// StartOrRun starts the existing container or runs a new one.
func (p *Proxy) StartOrRun() Line { return combine("||", p.Start(), p.Run()) }

// Info returns docker ps of kamal-proxy.
func (p *Proxy) Info() Line { return docker(w("ps", "--filter", "'name=^"+p.containerName()+"$'")) }

// Version prints the tag of the running proxy image.
func (p *Proxy) Version() Line {
	return pipe(docker(w("inspect", p.containerName(), "--format '{{.Config.Image}}'")), w("awk", "-F:", "'{print $NF}'"))
}

// Logs returns docker logs of kamal-proxy.
func (p *Proxy) Logs(opts LogOptions) Line {
	return pipe(docker(w("logs", p.containerName()), when(opts.Since != "", "--since "+opts.Since),
		when(opts.Lines != "", "--tail "+opts.Lines), when(opts.Timestamps, "--timestamps"), w("2>&1")), grepFilter(opts, "'"))
}

// FollowLogs returns the ssh command that follows the proxy logs on host.
func (p *Proxy) FollowLogs(host string, opts LogOptions) string {
	return p.RunOverSSH(pipe(docker(w("logs", p.containerName()), when(opts.Timestamps, "--timestamps"),
		w("--tail", "10", "--follow", "2>&1")), grepFilter(opts, `"`)), host)
}

// RemoveContainer prunes stopped kamal-proxy containers.
func (p *Proxy) RemoveContainer() Line {
	return docker(w("container", "prune", "--force", "--filter", "label=org.opencontainers.image.title=kamal-proxy"))
}

// RemoveImage prunes kamal-proxy images.
func (p *Proxy) RemoveImage() Line {
	return docker(w("image", "prune", "--all", "--force", "--filter", "label=org.opencontainers.image.title=kamal-proxy"))
}

// CleanupTraefik removes the Traefik container and images of Kamal 1.
func (p *Proxy) CleanupTraefik() Line {
	return chain(docker(w("container", "stop", "traefik")), combine("&&",
		docker(w("container", "prune", "--force", "--filter", "label=org.opencontainers.image.title=Traefik")),
		docker(w("image", "prune", "--all", "--force", "--filter", "label=org.opencontainers.image.title=Traefik"))))
}

// EnsureProxyDirectory returns mkdir -p .kamal/proxy.
func (p *Proxy) EnsureProxyDirectory() Line { return makeDirectory(p.cfg.ProxyBoot().HostDirectory()) }

// RemoveProxyDirectory returns rm -r .kamal/proxy.
func (p *Proxy) RemoveProxyDirectory() Line {
	return p.RemoveDirectory(p.cfg.ProxyBoot().HostDirectory())
}

// EnsureAppsConfigDirectory returns mkdir -p .kamal/proxy/apps-config.
func (p *Proxy) EnsureAppsConfigDirectory() Line {
	return makeDirectory(p.cfg.ProxyBoot().AppsDirectory())
}

// BootConfig echoes the boot options, image, version, and run command from the
// host's boot files, with defaults for missing files.
func (p *Proxy) BootConfig() Line {
	return w("echo", substitute(p.ReadBootOptions())+" "+substitute(p.ReadImage())+":"+substitute(p.ReadImageVersion())+" "+substitute(p.ReadRunCommand()))
}

// ReadBootOptions reads the options file, or the default publish and log flags.
func (p *Proxy) ReadBootOptions() Line {
	return readFile(p.cfg.ProxyBoot().OptionsFile(), strings.Join(p.cfg.ProxyBoot().DefaultBootOptions(), " "))
}

// ReadImage reads the image file, or basecamp/kamal-proxy.
func (p *Proxy) ReadImage() Line {
	return readFile(p.cfg.ProxyBoot().ImageFile(), p.cfg.ProxyBoot().ImageDefault())
}

// ReadImageVersion reads the image version file, or the minimum version.
func (p *Proxy) ReadImageVersion() Line {
	return readFile(p.cfg.ProxyBoot().ImageVersionFile(), configuration.ProxyMinimumVersion())
}

// ReadRunCommand reads the run command file, or nothing.
func (p *Proxy) ReadRunCommand() Line { return readFile(p.cfg.ProxyBoot().RunCommandFile(), "") }

// ResetBootOptions removes the options file.
func (p *Proxy) ResetBootOptions() Line { return p.RemoveFile(p.cfg.ProxyBoot().OptionsFile()) }

// ResetImage removes the image file.
func (p *Proxy) ResetImage() Line { return p.RemoveFile(p.cfg.ProxyBoot().ImageFile()) }

// ResetImageVersion removes the image version file.
func (p *Proxy) ResetImageVersion() Line { return p.RemoveFile(p.cfg.ProxyBoot().ImageVersionFile()) }

// ResetRunCommand removes the run command file.
func (p *Proxy) ResetRunCommand() Line { return p.RemoveFile(p.cfg.ProxyBoot().RunCommandFile()) }

func readFile(file, fallback string) Line {
	return combine("||", w("cat", file, "2>", "/dev/null"), w("echo", `"`+fallback+`"`))
}

func (p *Proxy) dockerRun() Line {
	return docker(w("run", "--name", p.containerName(), "--network", "kamal", "--detach", "--restart", "unless-stopped", "--volume", proxyConfigVolume),
		p.cfg.ProxyBoot().AppsVolume().DockerArgs())
}
