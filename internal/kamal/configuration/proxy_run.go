package configuration

// Ports lib/kamal/configuration/proxy/run.rb at Kamal 2.12.0.

import (
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// ProxyRun holds the settings of the kamal-proxy container itself.
type ProxyRun struct {
	cfg    *Configuration
	config *yamlord.Ordered
}

// Config returns the raw run settings.
func (r *ProxyRun) Config() *yamlord.Ordered { return r.config }

// Equal reports whether two run settings are the same, as ProxyRun#== does.
func (r *ProxyRun) Equal(other *ProxyRun) bool {
	return yamlord.Equal(yamlord.NewMap(r.config), yamlord.NewMap(other.config))
}

// Debug returns the debug setting, or nil.
func (r *ProxyRun) Debug() *yamlord.Value { return r.config.At("debug") }

// Publish reports whether the proxy publishes its ports, default true.
func (r *ProxyRun) Publish() bool { return r.config.Fetch("publish", yamlord.NewBool(true)).Truthy() }

// HTTPPort returns the host HTTP port, default 80.
func (r *ProxyRun) HTTPPort() *yamlord.Value {
	return r.config.Fetch("http_port", yamlord.NewInt(proxyDefaultHTTPPort))
}

// HTTPSPort returns the host HTTPS port, default 443.
func (r *ProxyRun) HTTPSPort() *yamlord.Value {
	return r.config.Fetch("https_port", yamlord.NewInt(proxyDefaultHTTPSPort))
}

// BindIPs returns the addresses the ports bind to, or nil for all.
func (r *ProxyRun) BindIPs() []string { return r.config.At("bind_ips").StringList() }

// PublishArgs returns the --publish flags, or "" when publish is false.
func (r *ProxyRun) PublishArgs() string {
	if !r.Publish() {
		return ""
	}
	return publishArgs(r.HTTPPort(), r.HTTPSPort(), r.BindIPs())
}

// LogMaxSize returns the maximum log size, default 10m.
func (r *ProxyRun) LogMaxSize() string {
	return r.config.Fetch("log_max_size", yamlord.NewString(proxyDefaultLogMaxSize)).ToS()
}

// LoggingArgs returns --log-opt max-size=<size>.
func (r *ProxyRun) LoggingArgs() shell.Line {
	if strings.TrimSpace(r.LogMaxSize()) == "" {
		return nil
	}
	return utils.Argumentize("--log-opt", utils.Keys("max-size="+r.LogMaxSize()), false)
}

// Version returns the proxy image tag, default v0.9.2.
func (r *ProxyRun) Version() string {
	return r.config.Fetch("version", yamlord.NewString(proxyMinimumVersion)).ToS()
}

// Registry returns the proxy image registry, or "".
func (r *ProxyRun) Registry() string { return r.config.At("registry").ToS() }

// Repository returns the proxy image repository, default basecamp/kamal-proxy.
func (r *ProxyRun) Repository() string {
	return r.config.Fetch("repository", yamlord.NewString(proxyDefaultImageRepo)).ToS()
}

// Image returns [registry/]repository:version.
func (r *ProxyRun) Image() string {
	return joinNonEmpty("/", r.Registry(), r.Repository()) + ":" + r.Version()
}

// ContainerName returns kamal-proxy.
func (r *ProxyRun) ContainerName() string { return ProxyContainerName }

// OptionsArgs returns the extra docker run options, or nil.
func (r *ProxyRun) OptionsArgs() shell.Line {
	options := r.config.At("options")
	if !options.Truthy() {
		return nil
	}
	return utils.Optionize(utils.AttrsOf(options.Map()))
}

// RunCommand returns "kamal-proxy run" with --debug and --metrics-port when set.
func (r *ProxyRun) RunCommand() string {
	return shell.Concat(shell.Words("kamal-proxy", "run"), utils.Optionize(r.RunCommandOptions())).String()
}

// MetricsPort returns the Prometheus port, or nil.
func (r *ProxyRun) MetricsPort() *yamlord.Value { return r.config.At("metrics_port") }

// RunCommandOptions returns debug and metrics-port when set.
func (r *ProxyRun) RunCommandOptions() []utils.Attr {
	var attrs []utils.Attr
	if r.Debug().Truthy() {
		attrs = append(attrs, utils.Attr{Key: "debug", Value: r.Debug()})
	}
	if !r.MetricsPort().IsNull() {
		attrs = append(attrs, utils.Attr{Key: "metrics-port", Value: r.MetricsPort()})
	}
	return attrs
}

// DockerOptionsArgs returns the volume, publish, logging, expose, and extra options
// of the proxy container.
func (r *ProxyRun) DockerOptionsArgs() []string {
	args := append([]string{r.AppsVolume().DockerArgs().String()}, nonEmpty(r.PublishArgs())...)
	args = append(args, wordsOf(r.LoggingArgs())...)
	if r.MetricsPort().Present() {
		args = append(args, "--expose="+r.MetricsPort().ToS())
	}
	return append(args, wordsOf(r.OptionsArgs())...)
}

func nonEmpty(text string) []string {
	if text == "" {
		return nil
	}
	return []string{text}
}

// AppsVolume returns the mount of the per-app proxy files, the same as
// ProxyBoot.AppsVolume. Kamal keeps a copy of these paths on both classes.
func (r *ProxyRun) AppsVolume() Volume { return r.cfg.ProxyBoot().AppsVolume() }

// AppDirectory returns this app's proxy directory on the host.
func (r *ProxyRun) AppDirectory() string { return r.cfg.ProxyBoot().AppDirectory() }

// AppContainerDirectory returns where AppDirectory appears in the container.
func (r *ProxyRun) AppContainerDirectory() string { return r.cfg.ProxyBoot().AppContainerDirectory() }
