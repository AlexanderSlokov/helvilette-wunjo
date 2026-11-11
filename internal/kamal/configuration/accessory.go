package configuration

// Ports lib/kamal/configuration/accessory.rb at Kamal 2.12.0.

import (
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/erb"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/kamalerr"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// accessoryDefaultNetwork is the Docker network that accessories join by default.
const accessoryDefaultNetwork = "kamal"

// Accessory is a supporting service, such as a database, that Kamal runs next to
// the app.
type Accessory struct {
	name     string
	cfg      *Configuration
	config   *yamlord.Ordered
	env      *Env
	proxy    *Proxy
	registry *Registry
}

func newAccessory(name string, cfg *Configuration) (*Accessory, error) {
	raw := cfg.raw.At("accessories").Map().At(name)
	context := "accessories/" + name
	if err := validateAccessory(raw, exampleDoc("accessory").Dig("accessories", "mysql"), context); err != nil {
		return nil, err
	}
	a := &Accessory{name: name, cfg: cfg, config: raw.Map()}
	if err := a.ensureValidRoles(); err != nil {
		return nil, err
	}
	if err := a.ensureValidPort(); err != nil {
		return nil, err
	}
	return a, a.initParts(context)
}

func (a *Accessory) initParts(context string) error {
	var err error
	if a.env, err = newEnv(a.config.Fetch("env", yamlord.NewMap(nil)), a.cfg.Secrets(), context+"/env"); err != nil {
		return err
	}
	if a.RunningProxy() {
		if a.proxy, err = newProxy(a.cfg, a.config.At("proxy"), "", a.cfg.Secrets(), context+"/proxy"); err != nil {
			return err
		}
	}
	if a.config.At("registry").Present() {
		a.registry, err = newRegistry(a.config, a.cfg.Secrets(), context+"/registry")
	}
	return err
}

// Name returns the accessory name, such as mysql.
func (a *Accessory) Name() string { return a.name }

// Env returns the accessory's variables.
func (a *Accessory) Env() *Env { return a.env }

// Proxy returns the accessory's proxy, or nil.
func (a *Accessory) Proxy() *Proxy { return a.proxy }

// Registry returns the accessory's own registry, or nil.
func (a *Accessory) Registry() *Registry { return a.registry }

// ServiceName returns the container name: the service setting, or <app>-<name>.
func (a *Accessory) ServiceName() string {
	if service := a.config.At("service"); service.Truthy() {
		return service.ToS()
	}
	return a.cfg.Service() + "-" + a.name
}

// Image returns [registry/]image.
func (a *Accessory) Image() string {
	server := ""
	if a.registry != nil {
		server = a.registry.Server()
	}
	return joinNonEmpty("/", server, a.config.At("image").ToS())
}

// Hosts returns the hosts from host, hosts, role, roles, tag, or tags.
func (a *Accessory) Hosts() []string {
	switch {
	case a.config.Has("host"):
		return []string{a.config.At("host").ToS()}
	case a.config.Has("hosts"):
		return a.config.At("hosts").StringList()
	case a.config.Has("role"):
		return a.roleHosts(a.config.At("role").ToS())
	case a.config.Has("roles"):
		return a.rolesHosts()
	default:
		return a.taggedHosts()
	}
}

func (a *Accessory) roleHosts(name string) []string {
	if role := a.cfg.Role(name); role != nil {
		return role.Hosts()
	}
	return nil
}

func (a *Accessory) rolesHosts() []string {
	var hosts []string
	for _, name := range a.config.At("roles").StringList() {
		hosts = append(hosts, a.roleHosts(name)...)
	}
	return hosts
}

func (a *Accessory) taggedHosts() []string {
	var tags []string
	if a.config.Has("tag") {
		tags = []string{a.config.At("tag").ToS()}
	} else {
		tags = a.config.At("tags").StringList()
	}
	var hosts []string
	for _, tag := range tags {
		hosts = append(hosts, hostsWithTag(a.cfg.raw.At("servers"), tag)...)
	}
	return hosts
}

// hostsWithTag returns the hosts tagged with tag in roles written as host lists. A
// tag written as one string matches as a substring, as Ruby's String#include? does.
func hostsWithTag(servers *yamlord.Value, tag string) []string {
	var hosts []string
	servers.Map().Each(func(_ string, entries *yamlord.Value) {
		for _, entry := range entries.List() {
			if entry.Kind() == yamlord.Map && tagMatches(entry.Map().At(entry.Map().Keys()[0]), tag) {
				hosts = append(hosts, entry.Map().Keys()[0])
			}
		}
	})
	return hosts
}

func tagMatches(tags *yamlord.Value, tag string) bool {
	if tags.Kind() == yamlord.List {
		return utils.Contains(tags.StringList(), tag)
	}
	return strings.Contains(tags.ToS(), tag)
}

// Port returns the publish setting as text, or "".
func (a *Accessory) Port() string {
	port := a.config.At("port").ToS()
	if strings.TrimSpace(port) == "" {
		return ""
	}
	return port
}

// NetworkArgs returns --network <network>, default kamal.
func (a *Accessory) NetworkArgs() shell.Line {
	return utils.Argumentize("--network", utils.Keys(a.network()), false)
}

// PublishArgs returns --publish <port>, or nil without a port.
func (a *Accessory) PublishArgs() shell.Line {
	if a.Port() == "" {
		return nil
	}
	return utils.Argumentize("--publish", utils.Keys(a.Port()), false)
}

// Labels returns service, then the custom labels.
func (a *Accessory) Labels() *yamlord.Ordered {
	defaults := yamlord.OrderedOf(yamlord.Pair{Key: "service", Value: yamlord.NewString(a.ServiceName())})
	if labels := a.config.At("labels").Map(); labels != nil {
		return defaults.Merge(labels)
	}
	return defaults
}

// LabelArgs returns --label flags.
func (a *Accessory) LabelArgs() shell.Line {
	return utils.Argumentize("--label", utils.AttrsOf(a.Labels()), false)
}

// EnvArgs returns --env flags and --env-file <secrets path>.
func (a *Accessory) EnvArgs() shell.Line {
	return shell.Concat(a.env.ClearArgs(), utils.Argumentize("--env-file", utils.Keys(a.SecretsPath()), false))
}

// EnvDirectory returns .kamal/apps/<service>/env/accessories.
func (a *Accessory) EnvDirectory() string { return filepath.Join(a.cfg.EnvDirectory(), "accessories") }

// SecretsFile returns the env file of the accessory's secrets.
func (a *Accessory) SecretsFile() (string, error) { return a.env.SecretsFile() }

// SecretsPath returns .kamal/apps/<service>/env/accessories/<name>.env.
func (a *Accessory) SecretsPath() string { return filepath.Join(a.EnvDirectory(), a.name+".env") }

// Cmd returns the accessory's command, or nil.
func (a *Accessory) Cmd() *yamlord.Value { return a.config.At("cmd") }

// RunningProxy reports whether the accessory runs behind kamal-proxy.
func (a *Accessory) RunningProxy() bool { return a.config.At("proxy").Present() }

// OptionArgs returns the docker run options without restart.
func (a *Accessory) OptionArgs() shell.Line { return optionArgsWithoutRestart(a.dockerOptions()) }

// RestartPolicy returns the restart option, default unless-stopped.
func (a *Accessory) RestartPolicy() string { return restartPolicyOf(a.dockerOptions()) }

func (a *Accessory) network() string {
	if network := a.config.At("network"); network.Truthy() {
		return network.ToS()
	}
	return accessoryDefaultNetwork
}

func (a *Accessory) dockerOptions() *yamlord.Ordered {
	if options := a.config.At("options").Map(); options != nil {
		return options
	}
	return yamlord.NewOrdered()
}

func (a *Accessory) ensureValidRoles() error {
	if roles := a.config.At("roles"); roles.Truthy() {
		return a.checkRolesExist(roles.StringList())
	}
	if role := a.config.At("role"); role.Truthy() && a.cfg.Role(role.ToS()) == nil {
		return kamalerr.Configuration(fmt.Sprintf("accessories/%s: unknown role %s", a.name, role.ToS()))
	}
	return nil
}

func (a *Accessory) checkRolesExist(roles []string) error {
	var known []string
	for _, role := range a.cfg.Roles() {
		known = append(known, role.Name())
	}
	if missing := subtract(roles, known); len(missing) > 0 {
		return kamalerr.Configuration(fmt.Sprintf("accessories/%s: unknown roles %s", a.name, strings.Join(missing, ", ")))
	}
	return nil
}

// ensureValidPort requires a bind address in port and in options/publish, so an
// accessory never listens on every interface by accident.
func (a *Accessory) ensureValidPort() error {
	if err := a.ensureBoundPublish(a.Port(), "port"); err != nil {
		return err
	}
	for _, publish := range a.dockerOptions().At("publish").StringList() {
		if err := a.ensureBoundPublish(publish, "options/publish"); err != nil {
			return err
		}
	}
	return nil
}

func (a *Accessory) ensureBoundPublish(portConfig, key string) error {
	if strings.TrimSpace(portConfig) == "" || publishHasBindAddress(portConfig) {
		return nil
	}
	return kamalerr.Configuration(fmt.Sprintf("accessories/%s: %s \"%s\" must name a bind address — "+
		"\"127.0.0.1:PORT:PORT\" to keep it private, \"0.0.0.0:PORT:PORT\" to publish it. "+
		"Run `kamal accessory reboot %s` to rebind an existing container.", a.name, key, portConfig, a.name))
}

func publishHasBindAddress(portConfig string) bool {
	host, _, _ := strings.Cut(portConfig, "/")
	if strings.HasPrefix(host, "[") {
		end := strings.Index(host, "]")
		return end > 1 && net.ParseIP(host[1:end]) != nil
	}
	if strings.Count(host, ":") >= 2 {
		return net.ParseIP(strings.Split(host, ":")[0]) != nil
	}
	return false
}

// AccessoryPath is one file or directory that Kamal copies or mounts into an
// accessory. Content holds a rendered .erb file; Local holds the local path of any
// other file or directory.
type AccessoryPath struct {
	Local         string
	Content       string
	Rendered      bool
	HostPath      string
	ContainerPath string
	Options       string
	Mode          string
	Owner         string
}

// Files returns the files to upload, with .erb files rendered.
func (a *Accessory) Files() ([]AccessoryPath, error) {
	var out []AccessoryPath
	for _, entry := range a.config.At("files").List() {
		path, err := a.fileFor(entry)
		if err != nil {
			return nil, err
		}
		out = addPath(out, path)
	}
	return out, nil
}

func (a *Accessory) fileFor(entry *yamlord.Value) (AccessoryPath, error) {
	path := parsePathConfig(entry, "755")
	remote := path.ContainerPath
	path.HostPath = a.ServiceName() + remote
	if !strings.HasSuffix(path.Local, "erb") {
		path.Local = a.cfg.absolutePath(path.Local)
		return path, nil
	}
	content, err := a.renderFile(path.Local)
	path.Content, path.Rendered = content, true
	return path, err
}

// renderFile renders an .erb file with the process environment and the accessory's
// env, without trim mode, as Kamal does.
func (a *Accessory) renderFile(local string) (string, error) {
	template, err := a.cfg.env.Files.ReadFile(local)
	if err != nil {
		return "", err
	}
	vars, err := a.env.ToH()
	if err != nil {
		return "", err
	}
	lookup := func(name string) (string, bool) {
		if value, ok := vars.Get(name); ok {
			return value.ToS(), true
		}
		return a.cfg.lookupEnv(name)
	}
	return erb.Render(string(template), erb.Options{File: local, Lookup: lookup})
}

// Directories returns the directories to create and mount.
func (a *Accessory) Directories() []AccessoryPath {
	var out []AccessoryPath
	for _, entry := range a.config.At("directories").List() {
		path := parsePathConfig(entry, "")
		local := path.Local
		path.Local = a.expandHostPath(local)
		path.HostPath = a.expandHostPathForVolume(local)
		out = addPath(out, path)
	}
	return out
}

// addPath keeps the first position of a repeated local path and the last settings,
// as a Ruby Hash keyed by the path does. Rendered files never repeat.
func addPath(paths []AccessoryPath, path AccessoryPath) []AccessoryPath {
	for i := range paths {
		if !path.Rendered && !paths[i].Rendered && paths[i].Local == path.Local {
			paths[i] = path
			return paths
		}
	}
	return append(paths, path)
}

// parsePathConfig reads "local:remote[:options]" or a Hash with local, remote, mode,
// owner, and options.
func parsePathConfig(entry *yamlord.Value, defaultMode string) AccessoryPath {
	if m := entry.Map(); m != nil {
		mode := m.At("mode").ToS()
		if !m.At("mode").Truthy() {
			mode = defaultMode
		}
		return AccessoryPath{Local: m.At("local").ToS(), ContainerPath: m.At("remote").ToS(), Options: m.At("options").ToS(), Mode: mode, Owner: m.At("owner").ToS()}
	}
	parts := strings.SplitN(entry.ToS(), ":", 3)
	for len(parts) < 3 {
		parts = append(parts, "")
	}
	return AccessoryPath{Local: parts[0], ContainerPath: parts[1], Options: parts[2], Mode: defaultMode}
}

// VolumeArgs returns --volume flags for volumes, files, and directories.
func (a *Accessory) VolumeArgs() (shell.Line, error) {
	line := utils.Argumentize("--volume", utils.KeysOf(a.config.At("volumes")), false)
	files, err := a.Files()
	if err != nil {
		return nil, err
	}
	for _, path := range append(files, a.Directories()...) {
		line = append(line, Volume{HostPath: path.HostPath, ContainerPath: path.ContainerPath, Options: path.Options}.DockerArgs()...)
	}
	return line, nil
}

func (a *Accessory) expandHostPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return rubyFileJoin("$PWD/"+a.ServiceName(), path)
}

func (a *Accessory) expandHostPathForVolume(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return rubyFileJoin(a.ServiceName(), path)
}
