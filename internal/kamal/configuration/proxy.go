package configuration

// Ports lib/kamal/configuration/proxy.rb at Kamal 2.12.0.

import (
	"path/filepath"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/secrets"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// ProxyContainerName is the name of the kamal-proxy container on each host.
const ProxyContainerName = "kamal-proxy"

var defaultLogRequestHeaders = []string{"Cache-Control", "Last-Modified", "User-Agent"}

// Proxy holds how kamal-proxy routes to a role or an accessory.
type Proxy struct {
	cfg      *Configuration
	config   *yamlord.Ordered
	roleName string
	secrets  *secrets.Secrets
	run      *ProxyRun
}

func newProxy(cfg *Configuration, config *yamlord.Value, roleName string, sec *secrets.Secrets, context string) (*Proxy, error) {
	if config.IsNull() {
		config = yamlord.NewMap(nil)
	}
	if err := validateProxy(config, example("proxy"), context); err != nil {
		return nil, err
	}
	p := &Proxy{cfg: cfg, config: config.Map(), roleName: roleName, secrets: sec}
	if run := p.config.At("run"); run.Present() {
		p.run = &ProxyRun{cfg: cfg, config: run.Map()}
	}
	return p, nil
}

// Config returns the raw proxy settings.
func (p *Proxy) Config() *yamlord.Ordered { return p.config }

// Run returns the kamal-proxy container settings, or nil when unset.
func (p *Proxy) Run() *ProxyRun { return p.run }

// AppPort returns the port the app listens on, default 80.
func (p *Proxy) AppPort() *yamlord.Value { return p.config.Fetch("app_port", yamlord.NewInt(80)) }

// SSL reports whether kamal-proxy terminates TLS.
func (p *Proxy) SSL() bool { return p.config.Fetch("ssl", yamlord.NewBool(false)).Truthy() }

// Hosts returns the hostnames that route to the app.
func (p *Proxy) Hosts() []string {
	if hosts := p.config.At("hosts"); hosts.Truthy() {
		return hosts.StringList()
	}
	if host := p.config.At("host"); host.Truthy() {
		return strings.Split(host.ToS(), ",")
	}
	return nil
}

// CustomSSLCertificate reports whether ssl names a certificate and a key.
func (p *Proxy) CustomSSLCertificate() bool {
	ssl := p.config.At("ssl")
	return ssl.Kind() == yamlord.Map && ssl.Dig("certificate_pem").Present() && ssl.Dig("private_key_pem").Present()
}

// CertificatePEMContent returns the certificate from the secrets, or "" without a
// custom certificate.
func (p *Proxy) CertificatePEMContent() (string, error) { return p.sslSecret("certificate_pem") }

// PrivateKeyPEMContent returns the key from the secrets, or "" without a custom
// certificate.
func (p *Proxy) PrivateKeyPEMContent() (string, error) { return p.sslSecret("private_key_pem") }

func (p *Proxy) sslSecret(key string) (string, error) {
	ssl := p.config.At("ssl")
	if ssl.Kind() != yamlord.Map {
		return "", nil
	}
	return p.secrets.Fetch(ssl.Dig(key).ToS())
}

// HostTLSCert returns the certificate path on the host, or "".
func (p *Proxy) HostTLSCert() string { return p.tlsPath(p.cfg.ProxyBoot().TLSDirectory(), "cert.pem") }

// HostTLSKey returns the key path on the host, or "".
func (p *Proxy) HostTLSKey() string { return p.tlsPath(p.cfg.ProxyBoot().TLSDirectory(), "key.pem") }

// ContainerTLSCert returns the certificate path in the proxy container, or "".
func (p *Proxy) ContainerTLSCert() string {
	return p.tlsPath(p.cfg.ProxyBoot().TLSContainerDirectory(), "cert.pem")
}

// ContainerTLSKey returns the key path in the proxy container, or "".
func (p *Proxy) ContainerTLSKey() string {
	return p.tlsPath(p.cfg.ProxyBoot().TLSContainerDirectory(), "key.pem")
}

func (p *Proxy) tlsPath(directory, filename string) string {
	if !p.CustomSSLCertificate() {
		return ""
	}
	parts := []string{directory}
	if p.roleName != "" {
		parts = append(parts, p.roleName)
	}
	return filepath.Join(append(parts, filename)...)
}

// PathPrefixes returns the path prefixes that route to the app.
func (p *Proxy) PathPrefixes() []string {
	if prefixes := p.config.At("path_prefixes"); prefixes.Truthy() {
		return prefixes.StringList()
	}
	if prefix := p.config.At("path_prefix"); prefix.Truthy() {
		return strings.Split(prefix.ToS(), ",")
	}
	return nil
}

// DeployOptions returns the options of `kamal-proxy deploy`, in Kamal's order, with
// unset options left out.
func (p *Proxy) DeployOptions() []utils.Attr {
	var attrs []utils.Attr
	add := func(key string, value *yamlord.Value) {
		if !value.IsNull() {
			attrs = append(attrs, utils.Attr{Key: key, Value: value})
		}
	}
	p.addRoutingOptions(add)
	p.addTimeoutOptions(add)
	p.addBufferingOptions(add)
	p.addPathAndLoggingOptions(add)
	return attrs
}

type addOption func(key string, value *yamlord.Value)

func (p *Proxy) addRoutingOptions(add addOption) {
	add("host", yamlord.Strings(p.Hosts()...))
	if p.SSL() {
		add("tls", yamlord.NewBool(true))
	}
	add("tls-certificate-path", optionalString(p.ContainerTLSCert()))
	add("tls-private-key-path", optionalString(p.ContainerTLSKey()))
}

func (p *Proxy) addTimeoutOptions(add addOption) {
	add("deploy-timeout", secondsDuration(p.cfg.DeployTimeout()))
	add("drain-timeout", secondsDuration(p.cfg.DrainTimeout()))
	add("health-check-interval", secondsDuration(p.dig("healthcheck", "interval")))
	add("health-check-timeout", secondsDuration(p.dig("healthcheck", "timeout")))
	add("health-check-path", p.dig("healthcheck", "path"))
	add("target-timeout", secondsDuration(p.config.At("response_timeout")))
}

func (p *Proxy) addBufferingOptions(add addOption) {
	add("buffer-requests", p.bufferingFlag("requests"))
	add("buffer-responses", p.bufferingFlag("responses"))
	add("buffer-memory", p.dig("buffering", "memory"))
	add("max-request-body", p.dig("buffering", "max_request_body"))
	add("max-response-body", p.dig("buffering", "max_response_body"))
}

func (p *Proxy) addPathAndLoggingOptions(add addOption) {
	add("path-prefix", yamlord.Strings(p.PathPrefixes()...))
	add("strip-path-prefix", p.config.At("strip_path_prefix"))
	add("forward-headers", p.config.At("forward_headers"))
	add("tls-redirect", p.config.At("ssl_redirect"))
	headers := p.dig("logging", "request_headers")
	if !headers.Truthy() {
		headers = yamlord.Strings(defaultLogRequestHeaders...)
	}
	add("log-request-header", headers)
	add("log-response-header", p.dig("logging", "response_headers"))
	add("error-pages", optionalString(p.errorPages()))
}

// bufferingFlag returns buffering[key], default true, as
// proxy_config.fetch("buffering", ...).fetch(key, true) does.
func (p *Proxy) bufferingFlag(key string) *yamlord.Value {
	buffering := p.config.At("buffering").Map()
	if buffering == nil {
		return yamlord.NewBool(true)
	}
	return buffering.Fetch(key, yamlord.NewBool(true))
}

func (p *Proxy) dig(keys ...string) *yamlord.Value { return yamlord.NewMap(p.config).Dig(keys...) }

// DeployCommandArgs returns --target=<target>:<app port> and the deploy options.
//
//	p.DeployCommandArgs("abc123") // --target="abc123:80" --host="app.example.com" ...
func (p *Proxy) DeployCommandArgs(target string) shell.Line {
	targetAttr := utils.Attr{Key: "target", Value: yamlord.NewString(target + ":" + p.AppPort().ToS())}
	return utils.OptionizeWith(append([]utils.Attr{targetAttr}, p.DeployOptions()...), "=")
}

// StopCommandArgs returns the options of `kamal-proxy stop`.
func (p *Proxy) StopCommandArgs(drainTimeout *yamlord.Value, message string) shell.Line {
	var attrs []utils.Attr
	if d := secondsDuration(drainTimeout); !d.IsNull() {
		attrs = append(attrs, utils.Attr{Key: "drain-timeout", Value: d})
	}
	if message != "" {
		attrs = append(attrs, utils.Attr{Key: "message", Value: yamlord.NewString(message)})
	}
	return utils.OptionizeWith(attrs, "=")
}

// Merge returns a proxy whose settings are other's deep-merged with p's, so p wins.
func (p *Proxy) Merge(other *Proxy) (*Proxy, error) {
	merged := other.config.DeepMerge(p.config)
	return newProxy(p.cfg, yamlord.NewMap(merged), p.roleName, p.secrets, "proxy")
}

func (p *Proxy) errorPages() string {
	if !p.cfg.ErrorPagesPath().Truthy() {
		return ""
	}
	return filepath.Join(p.cfg.ProxyBoot().ErrorPagesContainerDirectory(), p.cfg.Version())
}

func secondsDuration(value *yamlord.Value) *yamlord.Value {
	if !value.Truthy() {
		return nil
	}
	return yamlord.NewString(value.ToS() + "s")
}

func optionalString(text string) *yamlord.Value {
	if text == "" {
		return nil
	}
	return yamlord.NewString(text)
}
