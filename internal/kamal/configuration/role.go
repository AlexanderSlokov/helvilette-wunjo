package configuration

// Ports lib/kamal/configuration/role.rb at Kamal 2.12.0.

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/kamalerr"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// Role is a group of hosts that run the app with the same command and settings.
type Role struct {
	name               string
	cfg                *Configuration
	config             *yamlord.Value
	specializedEnv     *Env
	specializedLogging *Logging
	specializedProxy   *Proxy
	runningProxy       bool
	proxy              *Proxy
	envs               map[string]*Env
}

func newRole(name string, cfg *Configuration) (*Role, error) {
	r := &Role{name: name, cfg: cfg, config: roleConfig(cfg.raw.At("servers"), name), envs: map[string]*Env{}}
	context := "servers/" + name
	if err := validateRole(r.config, exampleDoc("role").Dig("servers", "workers"), context); err != nil {
		return nil, err
	}
	if err := r.initSpecializations(context); err != nil {
		return nil, err
	}
	return r, r.initSpecializedProxy(context)
}

func roleConfig(servers *yamlord.Value, name string) *yamlord.Value {
	if servers.Kind() == yamlord.List {
		return yamlord.NewMap(nil)
	}
	return servers.Map().At(name)
}

func (r *Role) initSpecializations(context string) error {
	var err error
	r.specializedEnv, err = newEnv(r.specializations().Fetch("env", yamlord.NewMap(nil)), r.cfg.Secrets(), context+"/env")
	if err != nil {
		return err
	}
	r.specializedLogging, err = newLogging(r.specializations().Fetch("logging", yamlord.NewMap(nil)), context+"/logging")
	return err
}

// initSpecializedProxy decides whether the role runs behind kamal-proxy: the primary
// role does unless proxy is false, and other roles do only when proxy is set.
func (r *Role) initSpecializedProxy(context string) error {
	spec := r.specializations().At("proxy")
	if r.IsPrimary() {
		r.runningProxy = !spec.IsFalse()
	} else {
		r.runningProxy = spec.Truthy()
	}
	if !r.runningProxy {
		return nil
	}
	if spec.IsTrue() || spec.IsNull() {
		spec = yamlord.NewMap(nil)
	}
	var err error
	r.specializedProxy, err = newProxy(r.cfg, spec, r.name, r.cfg.Secrets(), context+"/proxy")
	return err
}

// Name returns the role name, such as web.
func (r *Role) Name() string { return r.name }

func (r *Role) String() string { return r.name }

// PrimaryHost returns the first host, or "".
func (r *Role) PrimaryHost() string {
	if hosts := r.Hosts(); len(hosts) > 0 {
		return hosts[0]
	}
	return ""
}

// Hosts returns the hosts in file order without duplicates.
func (r *Role) Hosts() []string {
	hosts, _ := r.taggedHosts()
	return hosts
}

// EnvTags returns the env tags of the host's tags that the root env defines.
func (r *Role) EnvTags(host string) []*EnvTag {
	_, tagsByHost := r.taggedHosts()
	var out []*EnvTag
	for _, tag := range tagsByHost[host] {
		if envTag := r.cfg.EnvTag(tag); envTag != nil {
			out = append(out, envTag)
		}
	}
	return out
}

// Cmd returns the role's command, or nil.
func (r *Role) Cmd() *yamlord.Value { return r.specializations().At("cmd") }

// OptionArgs returns the docker run options without restart.
func (r *Role) OptionArgs() shell.Line { return optionArgsWithoutRestart(r.dockerOptions()) }

// RestartPolicy returns the restart option, default unless-stopped.
func (r *Role) RestartPolicy() string { return restartPolicyOf(r.dockerOptions()) }

// Labels returns service, role, destination, then the custom labels.
func (r *Role) Labels() *yamlord.Ordered {
	defaults := yamlord.OrderedOf(
		yamlord.Pair{Key: "service", Value: yamlord.NewString(r.cfg.Service())},
		yamlord.Pair{Key: "role", Value: yamlord.NewString(r.name)},
		yamlord.Pair{Key: "destination", Value: optionalString(r.cfg.Destination())},
	)
	return defaults.Merge(r.customLabels())
}

// LabelArgs returns --label flags.
func (r *Role) LabelArgs() shell.Line {
	return utils.Argumentize("--label", utils.AttrsOf(r.Labels()), false)
}

// LoggingArgs returns the log flags of the role.
func (r *Role) LoggingArgs() shell.Line { return r.Logging().Args() }

// Logging returns the root logging merged with the role's.
func (r *Role) Logging() *Logging { return r.cfg.Logging().Merge(r.specializedLogging) }

// Proxy returns the role's proxy merged over the root proxy, or nil when the role
// runs without kamal-proxy.
func (r *Role) Proxy() *Proxy { return r.proxy }

// resolveProxy computes the merged proxy once the root proxy exists.
func (r *Role) resolveProxy() error {
	if !r.runningProxy {
		return nil
	}
	merged, err := r.specializedProxy.Merge(r.cfg.Proxy())
	r.proxy = merged
	return err
}

// RunningProxy reports whether the role runs behind kamal-proxy.
func (r *Role) RunningProxy() bool { return r.runningProxy }

// SSL reports whether the role's proxy terminates TLS.
func (r *Role) SSL() bool { return r.runningProxy && r.proxy.SSL() }

// StopArgs returns -t <seconds>: the stop timeout, or the drain timeout for a role
// without kamal-proxy, which drains requests itself.
func (r *Role) StopArgs() shell.Line {
	timeout := r.StopTimeout()
	if timeout.IsNull() && !r.runningProxy {
		timeout = r.cfg.DrainTimeout()
	}
	return utils.Argumentize("-t", utils.KeysOf(timeout), false)
}

// StopTimeout returns the role's stop timeout, or the root one, or nil.
func (r *Role) StopTimeout() *yamlord.Value {
	if timeout := r.specializations().At("stop_timeout"); timeout.Truthy() {
		return timeout
	}
	return r.cfg.StopTimeout()
}

// Env returns the variables for host: root env, role env, then the host's env tags.
func (r *Role) Env(host string) (*Env, error) {
	if env, ok := r.envs[host]; ok {
		return env, nil
	}
	env := r.cfg.Env().Merge(r.specializedEnv)
	for _, tag := range r.EnvTags(host) {
		tagEnv, err := tag.Env()
		if err != nil {
			return nil, err
		}
		env = env.Merge(tagEnv)
	}
	r.envs[host] = env
	return env, nil
}

// EnvArgs returns --env flags and --env-file <secrets path> for host.
func (r *Role) EnvArgs(host string) (shell.Line, error) {
	env, err := r.Env(host)
	if err != nil {
		return nil, err
	}
	return shell.Concat(env.ClearArgs(), utils.Argumentize("--env-file", utils.Keys(r.SecretsPath()), false)), nil
}

// EnvDirectory returns .kamal/apps/<service>/env/roles.
func (r *Role) EnvDirectory() string { return filepath.Join(r.cfg.EnvDirectory(), "roles") }

// SecretsFile returns the env file of the host's secrets.
func (r *Role) SecretsFile(host string) (string, error) {
	env, err := r.Env(host)
	if err != nil {
		return "", err
	}
	return env.SecretsFile()
}

// SecretsPath returns .kamal/apps/<service>/env/roles/<role>.env.
func (r *Role) SecretsPath() string { return filepath.Join(r.EnvDirectory(), r.name+".env") }

// AssetVolumeArgs returns the --volume flag of the asset volume, or nil.
func (r *Role) AssetVolumeArgs() shell.Line {
	if volume, ok := r.AssetVolume(r.cfg.Version()); ok {
		return volume.DockerArgs()
	}
	return nil
}

// IsPrimary reports whether this is the primary role.
func (r *Role) IsPrimary() bool { return r.name == r.cfg.PrimaryRoleName() }

// ContainerName returns <service>-<role>[-<destination>]-<version>. An empty version
// means the configured one.
func (r *Role) ContainerName(version string) string {
	if version == "" {
		version = r.cfg.Version()
	}
	return r.ContainerPrefix() + "-" + version
}

// ContainerPrefix returns <service>-<role>[-<destination>].
func (r *Role) ContainerPrefix() string {
	return joinNonEmpty("-", r.cfg.Service(), r.name, r.cfg.Destination())
}

// AssetPath returns the path of assets in the container, or "".
func (r *Role) AssetPath() string {
	path, _ := r.assetPathConfig()
	return path
}

// AssetPathOptions returns the mount options after the colon of asset_path, or "".
func (r *Role) AssetPathOptions() string {
	_, options := r.assetPathConfig()
	return options
}

// Assets reports whether the role shares assets between releases.
func (r *Role) Assets() bool { return strings.TrimSpace(r.AssetPath()) != "" && r.runningProxy }

// AssetVolume returns the asset volume of version, and false without assets.
func (r *Role) AssetVolume(version string) (Volume, bool) {
	if !r.Assets() {
		return Volume{}, false
	}
	_, options := r.assetPathConfig()
	return Volume{HostPath: r.AssetVolumeDirectory(version), ContainerPath: r.AssetPath(), Options: options}, true
}

// AssetExtractedDirectory returns where the assets of version are extracted.
func (r *Role) AssetExtractedDirectory(version string) string {
	return filepath.Join(r.cfg.AssetsDirectory(), "extracted", r.name+"-"+version)
}

// AssetVolumeDirectory returns the host directory of the asset volume of version.
func (r *Role) AssetVolumeDirectory(version string) string {
	return filepath.Join(r.cfg.AssetsDirectory(), "volumes", r.name+"-"+version)
}

func (r *Role) ensureOneHostForSSL() error {
	if r.runningProxy && r.proxy.SSL() && len(r.Hosts()) > 1 && !r.proxy.CustomSSLCertificate() {
		return kamalerr.Configuration(fmt.Sprintf("SSL is only supported on a single server unless you provide custom certificates, found %d servers for role %s", len(r.Hosts()), r.name))
	}
	return nil
}

// taggedHosts returns the hosts in order and the tags of each host.
func (r *Role) taggedHosts() ([]string, map[string][]string) {
	var hosts []string
	tags := map[string][]string{}
	for _, entry := range r.hostEntries() {
		host, hostTags, ok := hostAndTags(entry)
		if !ok {
			continue
		}
		if _, seen := tags[host]; !seen {
			hosts = append(hosts, host)
		}
		tags[host] = hostTags
	}
	return hosts, tags
}

func hostAndTags(entry *yamlord.Value) (string, []string, bool) {
	switch entry.Kind() {
	case yamlord.Map:
		host := entry.Map().Keys()[0]
		return host, entry.Map().At(host).StringList(), true
	case yamlord.String:
		return entry.Text(), []string{}, true
	default:
		return "", nil, false
	}
}

func (r *Role) hostEntries() []*yamlord.Value {
	servers := r.cfg.raw.At("servers")
	if servers.Kind() == yamlord.List {
		return servers.List()
	}
	own := servers.Map().At(r.name)
	if own.Kind() == yamlord.List {
		return own.List()
	}
	return own.Dig("hosts").Array()
}

func (r *Role) specializations() *yamlord.Ordered {
	if r.config.Kind() == yamlord.Map {
		return r.config.Map()
	}
	return yamlord.NewOrdered()
}

func (r *Role) dockerOptions() *yamlord.Ordered {
	if options := r.specializations().At("options").Map(); options != nil {
		return options
	}
	return yamlord.NewOrdered()
}

func (r *Role) customLabels() *yamlord.Ordered {
	labels := yamlord.NewOrdered()
	if root := r.cfg.raw.At("labels"); root.Present() {
		labels = labels.Merge(root.Map())
	}
	if own := r.specializations().At("labels"); own.Present() {
		labels = labels.Merge(own.Map())
	}
	return labels
}

// assetPathConfig splits "<container path>[:<options>]" from the role or root
// asset_path.
func (r *Role) assetPathConfig() (string, string) {
	raw := r.specializations().At("asset_path")
	if !raw.Truthy() {
		raw = r.cfg.raw.At("asset_path")
	}
	if !raw.Present() {
		return "", ""
	}
	path, options, _ := strings.Cut(raw.ToS(), ":")
	return path, options
}

// optionArgsWithoutRestart returns optionize of options without restart, which
// Kamal passes as --restart separately.
func optionArgsWithoutRestart(options *yamlord.Ordered) shell.Line {
	var attrs []utils.Attr
	for _, attr := range utils.AttrsOf(options) {
		if attr.Key != "restart" {
			attrs = append(attrs, attr)
		}
	}
	return utils.Optionize(attrs)
}

func restartPolicyOf(options *yamlord.Ordered) string {
	if policy, ok := options.Get("restart"); ok && policy.Truthy() {
		return policy.ToS()
	}
	return "unless-stopped"
}
