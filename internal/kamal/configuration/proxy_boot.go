package configuration

// Ports lib/kamal/configuration/proxy/boot.rb at Kamal 2.12.0.

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

const (
	proxyMinimumVersion     = "v0.9.2"
	proxyDefaultHTTPPort    = 80
	proxyDefaultHTTPSPort   = 443
	proxyDefaultLogMaxSize  = "10m"
	proxyAppsContainerDir   = "/home/kamal-proxy/.apps-config"
	proxyDefaultImageRepo   = "basecamp/kamal-proxy"
	proxyDefaultImageVendor = "basecamp"
)

// ProxyMinimumVersion is the oldest kamal-proxy that Kamal 2.12.0 supports.
func ProxyMinimumVersion() string { return proxyMinimumVersion }

// ProxyBoot holds the host paths and defaults of the kamal-proxy container.
type ProxyBoot struct {
	cfg *Configuration
}

// PublishArgs returns --publish flags for each bind IP, or for all interfaces.
func (b *ProxyBoot) PublishArgs(httpPort, httpsPort *yamlord.Value, bindIPs []string) (string, error) {
	for _, ip := range bindIPs {
		if !isIPAddress(ip) {
			return "", fmt.Errorf("Invalid publish IP address: %s", ip)
		}
	}
	return publishArgs(httpPort, httpsPort, bindIPs), nil
}

func publishArgs(httpPort, httpsPort *yamlord.Value, bindIPs []string) string {
	if len(bindIPs) == 0 {
		bindIPs = []string{""}
	}
	parts := make([]string, len(bindIPs))
	for i, ip := range bindIPs {
		ip = formatBindIP(ip)
		http := joinNonEmpty(":", ip, httpPort.ToS(), strconv.Itoa(proxyDefaultHTTPPort))
		https := joinNonEmpty(":", ip, httpsPort.ToS(), strconv.Itoa(proxyDefaultHTTPSPort))
		parts[i] = utils.Argumentize("--publish", utils.Keys(http, https), false).String()
	}
	return strings.Join(parts, " ")
}

var bracketed = regexp.MustCompile(`\A\[.*\]\z`)

// formatBindIP puts an IPv6 address in square brackets, such as [::1].
func formatBindIP(ip string) string {
	parsed := strings.Contains(ip, ":") && isIPAddress(ip)
	if parsed && !bracketed.MatchString(ip) {
		return "[" + ip + "]"
	}
	return ip
}

func joinNonEmpty(sep string, parts ...string) string {
	var kept []string
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, sep)
}

// LoggingArgs returns --log-opt max-size=<size>, or nothing for an empty size.
func (b *ProxyBoot) LoggingArgs(maxSize string) shell.Line {
	if strings.TrimSpace(maxSize) == "" {
		return nil
	}
	return utils.Argumentize("--log-opt", utils.Keys("max-size="+maxSize), false)
}

// DefaultBootOptions returns the publish and logging flags of a default proxy.
func (b *ProxyBoot) DefaultBootOptions() []string {
	defaults := publishArgs(yamlord.NewInt(proxyDefaultHTTPPort), yamlord.NewInt(proxyDefaultHTTPSPort), nil)
	return append([]string{defaults}, wordsOf(b.LoggingArgs(proxyDefaultLogMaxSize))...)
}

func wordsOf(line shell.Line) []string {
	out := make([]string, len(line))
	for i, word := range line {
		out[i] = word.String()
	}
	return out
}

// RepositoryName returns the owner of the default image.
func (b *ProxyBoot) RepositoryName() string { return proxyDefaultImageVendor }

// ImageName returns the name of the default image.
func (b *ProxyBoot) ImageName() string { return "kamal-proxy" }

// ImageDefault returns basecamp/kamal-proxy.
func (b *ProxyBoot) ImageDefault() string { return b.RepositoryName() + "/" + b.ImageName() }

// ContainerName returns kamal-proxy.
func (b *ProxyBoot) ContainerName() string { return ProxyContainerName }

// HostDirectory returns .kamal/proxy, where the proxy keeps its files on each host.
func (b *ProxyBoot) HostDirectory() string { return filepath.Join(b.cfg.RunDirectory(), "proxy") }

// OptionsFile returns the file that holds the proxy's docker run options.
func (b *ProxyBoot) OptionsFile() string { return filepath.Join(b.HostDirectory(), "options") }

// ImageFile returns the file that holds the proxy image.
func (b *ProxyBoot) ImageFile() string { return filepath.Join(b.HostDirectory(), "image") }

// ImageVersionFile returns the file that holds the proxy image version.
func (b *ProxyBoot) ImageVersionFile() string {
	return filepath.Join(b.HostDirectory(), "image_version")
}

// RunCommandFile returns the file that holds the proxy run command.
func (b *ProxyBoot) RunCommandFile() string { return filepath.Join(b.HostDirectory(), "run_command") }

// AppsDirectory returns the host directory of the per-app proxy files.
func (b *ProxyBoot) AppsDirectory() string { return filepath.Join(b.HostDirectory(), "apps-config") }

// AppsContainerDirectory returns where AppsDirectory appears in the container.
func (b *ProxyBoot) AppsContainerDirectory() string { return proxyAppsContainerDir }

// AppsVolume returns the mount of AppsDirectory into the container.
func (b *ProxyBoot) AppsVolume() Volume {
	return Volume{HostPath: b.AppsDirectory(), ContainerPath: b.AppsContainerDirectory()}
}

// AppDirectory returns the host directory of this app's proxy files.
func (b *ProxyBoot) AppDirectory() string {
	return filepath.Join(b.AppsDirectory(), b.cfg.ServiceAndDestination())
}

// AppContainerDirectory returns where AppDirectory appears in the container.
func (b *ProxyBoot) AppContainerDirectory() string {
	return filepath.Join(b.AppsContainerDirectory(), b.cfg.ServiceAndDestination())
}

// ErrorPagesDirectory returns the host directory of the app's error pages.
func (b *ProxyBoot) ErrorPagesDirectory() string {
	return filepath.Join(b.AppDirectory(), "error_pages")
}

// ErrorPagesContainerDirectory returns where ErrorPagesDirectory appears in the
// container.
func (b *ProxyBoot) ErrorPagesContainerDirectory() string {
	return filepath.Join(b.AppContainerDirectory(), "error_pages")
}

// TLSDirectory returns the host directory of the app's custom certificates.
func (b *ProxyBoot) TLSDirectory() string { return filepath.Join(b.AppDirectory(), "tls") }

// TLSContainerDirectory returns where TLSDirectory appears in the container.
func (b *ProxyBoot) TLSContainerDirectory() string {
	return filepath.Join(b.AppContainerDirectory(), "tls")
}
