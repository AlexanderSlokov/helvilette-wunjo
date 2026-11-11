package configuration

// Ports test/configuration_test.rb at Kamal 2.12.0.

import (
	"errors"
	"regexp"
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/kamalerr"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/testsupport"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

const withRoles = `
servers:
  web: [1.1.1.1, 1.1.1.2]
  workers:
    hosts: [1.1.1.1, 1.1.1.3]
`

func baseConfig(t *testing.T) *Configuration {
	return mustBuild(t, newWorld(), parseYAML(t, baseDeploy))
}

func rolesConfig(t *testing.T) *Configuration {
	return mustBuild(t, newWorld(), merged(t, baseDeploy, withRoles))
}

func assertConfigurationError(t *testing.T, err error) {
	t.Helper()
	var target *kamalerr.ConfigurationError
	if !errors.As(err, &target) {
		t.Fatalf("error = %v, want a ConfigurationError", err)
	}
}

func TestRequiredKeys(t *testing.T) {
	for _, key := range []string{"service", "image", "registry"} {
		raw := parseYAML(t, baseDeploy)
		raw.Delete(key)
		_, err := newWorld().build(raw)
		assertConfigurationError(t, err)
	}
}

func TestRegistryCredentialsRequired(t *testing.T) {
	for _, key := range []string{"username", "password"} {
		raw := parseYAML(t, baseDeploy)
		raw.At("registry").Map().Delete(key)
		_, err := newWorld().build(raw)
		assertConfigurationError(t, err)
	}
}

func TestImageUsesServiceNameIfRegistryIsLocal(t *testing.T) {
	raw := merged(t, baseDeploy, "registry: {server: 'localhost:5000'}")
	raw.Delete("image")
	if got := mustBuild(t, newWorld(), raw).Image(); got != "app" {
		t.Fatalf("image = %q", got)
	}
}

func TestImageUsesImageIfRegistryIsLocal(t *testing.T) {
	raw := merged(t, baseDeploy, "registry: {server: 'localhost:5000'}")
	if got := mustBuild(t, newWorld(), raw).Image(); got != "dhh/app" {
		t.Fatalf("image = %q", got)
	}
}

func TestServiceNameValid(t *testing.T) {
	for _, name := range []string{"hey-app1_primary", "MyApp"} {
		mustBuild(t, newWorld(), merged(t, baseDeploy, "service: "+name))
	}
}

func TestServiceNameInvalid(t *testing.T) {
	_, err := newWorld().build(merged(t, baseDeploy, "service: app.com"))
	assertConfigurationError(t, err)
}

func TestServersRequired(t *testing.T) {
	raw := parseYAML(t, baseDeploy)
	raw.Delete("servers")
	_, err := newWorld().build(raw)
	assertConfigurationError(t, err)
}

func TestServersNotRequiredWithAccessories(t *testing.T) {
	raw := merged(t, baseDeploy, "accessories: {foo: {image: foo/bar, host: 1.1.1.1}}")
	raw.Delete("servers")
	mustBuild(t, newWorld(), raw)
}

func TestRoles(t *testing.T) {
	if got := roleNamesOf(baseConfig(t)); got != "web" {
		t.Fatalf("roles = %s", got)
	}
	if got := roleNamesOf(rolesConfig(t)); got != "web workers" {
		t.Fatalf("roles = %s", got)
	}
}

func roleNamesOf(cfg *Configuration) string {
	var names []string
	for _, role := range cfg.Roles() {
		names = append(names, role.Name())
	}
	return joined(names)
}

func TestRole(t *testing.T) {
	if baseConfig(t).Role("web").Name() != "web" || rolesConfig(t).Role("workers").Name() != "workers" {
		t.Fatal("role lookup failed")
	}
	if baseConfig(t).Role("missing") != nil {
		t.Fatal("missing role found")
	}
}

func TestAllHosts(t *testing.T) {
	if got := joined(baseConfig(t).AllHosts()); got != "1.1.1.1 1.1.1.2" {
		t.Fatalf("hosts = %s", got)
	}
	if got := joined(rolesConfig(t).AllHosts()); got != "1.1.1.1 1.1.1.2 1.1.1.3" {
		t.Fatalf("hosts = %s", got)
	}
}

func TestPrimaryHost(t *testing.T) {
	if baseConfig(t).PrimaryHost() != "1.1.1.1" || rolesConfig(t).PrimaryHost() != "1.1.1.1" {
		t.Fatal("wrong primary host")
	}
}

func TestProxyHosts(t *testing.T) {
	if got := joined(rolesConfig(t).ProxyHosts()); got != "1.1.1.1 1.1.1.2" {
		t.Fatalf("proxy hosts = %s", got)
	}
	cfg := mustBuild(t, newWorld(), deepMerged(t, baseDeploy, withRoles, "servers: {workers: {proxy: true}}"))
	if got := joined(cfg.ProxyHosts()); got != "1.1.1.1 1.1.1.2 1.1.1.3" {
		t.Fatalf("proxy hosts = %s", got)
	}
}

func TestVersionNoGitRepo(t *testing.T) {
	w := newWorld()
	delete(w.env, "VERSION")
	w.results["git rev-parse"] = localshell.Result{Success: false}
	_, err := mustBuild(t, w, parseYAML(t, baseDeploy)).ResolveVersion()
	expectErrorContaining(t, err, "no git repository found")
}

func gitWorld(changes string) *fakeWorld {
	w := newWorld()
	delete(w.env, "VERSION")
	w.results["git rev-parse"] = localshell.Result{Success: true}
	w.results["git rev-parse HEAD"] = localshell.Result{Stdout: "git-version\n", Success: true}
	w.results["git status --porcelain"] = localshell.Result{Stdout: changes, Success: true}
	return w
}

func TestVersionFromGitCommitted(t *testing.T) {
	if got := mustBuild(t, gitWorld(""), parseYAML(t, baseDeploy)).Version(); got != "git-version" {
		t.Fatalf("version = %q", got)
	}
}

func TestVersionFromGitUncommitted(t *testing.T) {
	if got := mustBuild(t, gitWorld("M   file\n"), parseYAML(t, baseDeploy)).Version(); got != "git-version" {
		t.Fatalf("version = %q", got)
	}
}

func TestVersionFromUncommittedContext(t *testing.T) {
	cfg := mustBuild(t, gitWorld("M   file\n"), deepMerged(t, baseDeploy, "builder: {context: .}"))
	if got := cfg.Version(); !regexp.MustCompile(`^git-version_uncommitted_[0-9a-f]{16}$`).MatchString(got) {
		t.Fatalf("version = %q", got)
	}
}

func TestVersionFromEnv(t *testing.T) {
	w := newWorld()
	w.env["VERSION"] = "env-version"
	if got := mustBuild(t, w, parseYAML(t, baseDeploy)).Version(); got != "env-version" {
		t.Fatalf("version = %q", got)
	}
}

func TestVersionFromArg(t *testing.T) {
	cfg := baseConfig(t)
	cfg.SetVersion("arg-version")
	if cfg.Version() != "arg-version" {
		t.Fatalf("version = %q", cfg.Version())
	}
}

func TestRepositoryAndAbsoluteImage(t *testing.T) {
	cfg := baseConfig(t)
	if cfg.Repository() != "dhh/app" || cfg.AbsoluteImage() != "dhh/app:missing" {
		t.Fatalf("repository %q image %q", cfg.Repository(), cfg.AbsoluteImage())
	}
	ghcr := mustBuild(t, newWorld(), deepMerged(t, baseDeploy, "registry: {server: ghcr.io}"))
	if ghcr.Repository() != "ghcr.io/dhh/app" || ghcr.AbsoluteImage() != "ghcr.io/dhh/app:missing" {
		t.Fatalf("repository %q image %q", ghcr.Repository(), ghcr.AbsoluteImage())
	}
}

func TestServiceWithVersion(t *testing.T) {
	if got := baseConfig(t).ServiceWithVersion(); got != "app-missing" {
		t.Fatalf("got %q", got)
	}
}

func TestHostsRequiredForAllRoles(t *testing.T) {
	cases := []string{
		"servers: []",
		"servers: {web: []}",
		"servers: {web: {}}",
		"servers: {web: {hosts: []}}",
		"servers: {web: {hosts: null}}",
		"servers: {web: [web], workers: {hosts: []}}",
	}
	for _, override := range cases {
		_, err := newWorld().build(merged(t, baseDeploy, override))
		assertConfigurationError(t, err)
	}
}

func TestAllowEmptyRoles(t *testing.T) {
	mustBuild(t, newWorld(), merged(t, baseDeploy, "servers: {web: [web], workers: {hosts: []}}", "allow_empty_roles: true"))
	_, err := newWorld().build(merged(t, baseDeploy, "servers: {web: [], workers: {hosts: []}}", "allow_empty_roles: true"))
	assertConfigurationError(t, err)
}

func TestVolumeArgs(t *testing.T) {
	if got := joined(baseConfig(t).VolumeArgs()); got != "--volume /local/path:/container/path" {
		t.Fatalf("got %s", got)
	}
}

func TestLoggingArgs(t *testing.T) {
	cases := map[string]string{
		"": `--log-opt max-size="10m"`,
		"logging: {options: {max-size: 100m, max-file: 5}}":                `--log-opt max-size="100m" --log-opt max-file="5"`,
		"logging: {driver: local, options: {max-size: 100m, max-file: 5}}": `--log-driver "local" --log-opt max-size="100m" --log-opt max-file="5"`,
	}
	for override, want := range cases {
		cfg := mustBuild(t, newWorld(), merged(t, baseDeploy, override+"\n"))
		if got := joined(cfg.LoggingArgs()); got != want {
			t.Errorf("%s: got %s, want %s", override, got, want)
		}
	}
}

// fixtureWorld stores the named fixtures under fixtures/, with ArchHelper replaced.
func fixtureWorld(t *testing.T, names ...string) *fakeWorld {
	t.Helper()
	w := newWorld()
	w.env[testsupport.RemoteArchVariable] = testsupport.RemoteArch
	for _, name := range names {
		content, err := testsupport.ReadFixture(name)
		if err != nil {
			t.Fatal(err)
		}
		w.files["fixtures/"+name] = content
	}
	return w
}

func createFrom(t *testing.T, w *fakeWorld, file, destination string) (*Configuration, error) {
	t.Helper()
	return CreateFrom("fixtures/"+file, Options{Destination: destination, Env: w.environment()})
}

func TestERBEvaluationOfYmlConfig(t *testing.T) {
	cfg, err := createFrom(t, fixtureWorld(t, "deploy.erb.yml"), "deploy.erb.yml", "")
	if err != nil {
		t.Fatal(err)
	}
	if user, _ := cfg.Registry().Username(); user != "my-user" {
		t.Fatalf("username = %q", user)
	}
}

func TestDestinationIsLoadedIntoEnv(t *testing.T) {
	w := fixtureWorld(t, "deploy_for_dest.yml", "deploy_for_dest.world.yml")
	w.files["fixtures/deploy_for_dest.world.yml"] += "\nlabels:\n  dest: <%= ENV[\"KAMAL_DESTINATION\"] %>\n"
	cfg, err := createFrom(t, w, "deploy_for_dest.yml", "world")
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Labels().Dig("dest").ToS(); got != "world" {
		t.Fatalf("KAMAL_DESTINATION rendered as %q", got)
	}
}

func TestDestinationYmlConfigMerge(t *testing.T) {
	w := fixtureWorld(t, "deploy_for_dest.yml", "deploy_for_dest.world.yml", "deploy_for_dest.mars.yml")
	for destination, host := range map[string]string{"world": "1.1.1.1", "mars": "1.1.1.3"} {
		cfg, err := createFrom(t, w, "deploy_for_dest.yml", destination)
		if err != nil || cfg.AllHosts()[0] != host {
			t.Fatalf("%s: %v %v", destination, cfg, err)
		}
	}
}

func TestDestinationYmlConfigFileMissing(t *testing.T) {
	_, err := createFrom(t, fixtureWorld(t, "deploy_for_dest.yml"), "deploy_for_dest.yml", "missing")
	expectError(t, err, "Configuration file not found in fixtures/deploy_for_dest.missing.yml")
}

func TestDestinationRequired(t *testing.T) {
	w := fixtureWorld(t, "deploy_for_required_dest.yml", "deploy_for_required_dest.world.yml")
	if _, err := createFrom(t, w, "deploy_for_required_dest.yml", ""); !errors.Is(err, ErrDestinationRequired) {
		t.Fatalf("error = %v", err)
	}
	if _, err := createFrom(t, w, "deploy_for_required_dest.yml", "world"); err != nil {
		t.Fatal(err)
	}
}

func TestToH(t *testing.T) {
	h, err := baseConfig(t).ToH()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"roles" => ["web"], "hosts" => ["1.1.1.1", "1.1.1.2"], "primary_host" => "1.1.1.1", "version" => "missing", ` +
		`"repository" => "dhh/app", "absolute_image" => "dhh/app:missing", "service_with_version" => "app-missing", ` +
		`"volume_args" => ["--volume", "/local/path:/container/path"], ` +
		`"ssh_options" => {"user" => "root", "port" => 22, "keepalive" => true, "keepalive_interval" => 30, "log_level" => :fatal}, ` +
		`"sshkit" => {}, "builder" => {"arch" => "amd64"}, "logging" => ["--log-opt", "max-size=\"10m\""]}`
	if got := yamlord.NewMap(h).Inspect(); got != want {
		t.Fatalf("to_h =\n%s\nwant\n%s", got, want)
	}
}

func TestMinimumVersion(t *testing.T) {
	for _, version := range []string{"0.0.1", KamalVersion} {
		if got := mustBuild(t, newWorld(), merged(t, baseDeploy, "minimum_version: "+version)).MinimumVersion(); got != version {
			t.Fatalf("minimum version = %q", got)
		}
	}
	_, err := newWorld().build(merged(t, baseDeploy, "minimum_version: 10000.0.0"))
	assertConfigurationError(t, err)
}

func TestRunDirectory(t *testing.T) {
	if baseConfig(t).RunDirectory() != ".kamal" {
		t.Fatal("wrong run directory")
	}
}

func TestAssetPath(t *testing.T) {
	if !baseConfig(t).AssetPath().IsNull() {
		t.Fatal("asset path set by default")
	}
	if got := mustBuild(t, newWorld(), merged(t, baseDeploy, "asset_path: foo")).AssetPath().ToS(); got != "foo" {
		t.Fatalf("asset path = %q", got)
	}
}

func TestPrimaryRole(t *testing.T) {
	if baseConfig(t).PrimaryRole().Name() != "web" {
		t.Fatal("wrong primary role")
	}
	cfg := mustBuild(t, newWorld(), deepMerged(t, baseDeploy, withRoles,
		"servers: {alternate_web: {hosts: [1.1.1.4, 1.1.1.5]}}\nprimary_role: alternate_web"))
	role := cfg.Role("alternate_web")
	if cfg.PrimaryRole().Name() != "alternate_web" || cfg.PrimaryHost() != "1.1.1.4" || !role.IsPrimary() || !role.RunningProxy() {
		t.Fatal("alternate primary role not applied")
	}
}

func TestPrimaryRoleMissing(t *testing.T) {
	_, err := newWorld().build(merged(t, baseDeploy, "primary_role: bar"))
	expectErrorContaining(t, err, "bar isn't defined")
}

func TestRetainContainers(t *testing.T) {
	if baseConfig(t).RetainContainers() != 5 {
		t.Fatal("wrong default")
	}
	if got := mustBuild(t, newWorld(), merged(t, baseDeploy, withRoles, "retain_containers: 2")).RetainContainers(); got != 2 {
		t.Fatalf("retain = %d", got)
	}
	_, err := newWorld().build(merged(t, baseDeploy, withRoles, "retain_containers: 0"))
	assertConfigurationError(t, err)
}

func TestExtensions(t *testing.T) {
	cfg, err := createFrom(t, fixtureWorld(t, "deploy_with_extensions.yml"), "deploy_with_extensions.yml", "")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Role("web_tokyo").RunningProxy() || !cfg.Role("web_chicago").RunningProxy() {
		t.Fatal("extension roles do not run the proxy")
	}
}

func TestTraefikHooksRaiseError(t *testing.T) {
	w := newWorld()
	w.files[".kamal/hooks/post-traefik-reboot"] = ""
	w.files[".kamal/hooks/pre-traefik-reboot"] = ""
	_, err := w.build(parseYAML(t, baseDeploy))
	expectError(t, err, "Found pre-traefik-reboot, post-traefik-reboot, these should be renamed to (pre|post)-proxy-reboot")
}

func TestProxySSLRolesWithNoHost(t *testing.T) {
	_, err := newWorld().build(deepMerged(t, baseDeploy, withRoles, "servers: {workers: {proxy: {ssl: true}}}"))
	expectError(t, err, "servers/workers/proxy: Must set a host to enable automatic SSL")
}

func TestProxySSLRolesWithMultipleServers(t *testing.T) {
	_, err := newWorld().build(deepMerged(t, baseDeploy, withRoles, "servers: {workers: {proxy: {ssl: true, host: foo.example.com}}}"))
	expectError(t, err, "SSL is only supported on a single server unless you provide custom certificates, found 2 servers for role workers")
}

func TestTwoProxySSLRolesWithSameHost(t *testing.T) {
	servers := `servers:
  web: {hosts: [1.1.1.1], proxy: {ssl: true, host: foo.example.com}}
  workers: {hosts: [1.1.1.1], proxy: {ssl: true, host: foo.example.com}}`
	_, err := newWorld().build(merged(t, baseDeploy, servers))
	expectError(t, err, "Different roles can't share the same host for SSL: foo.example.com")
}

func TestTwoProxySSLRolesWithSameHostInAHostsArray(t *testing.T) {
	servers := `servers:
  web: {hosts: [1.1.1.1], proxy: {ssl: true, hosts: [foo.example.com, bar.example.com]}}
  workers: {hosts: [1.1.1.1], proxy: {ssl: true, hosts: [www.example.com, foo.example.com]}}`
	_, err := newWorld().build(merged(t, baseDeploy, servers))
	expectError(t, err, "Different roles can't share the same host for SSL: foo.example.com")
}

func TestHooksOutput(t *testing.T) {
	if baseConfig(t).HooksOutputFor("pre-deploy") != "" {
		t.Fatal("hooks_output set by default")
	}
	global := mustBuild(t, newWorld(), merged(t, baseDeploy, "hooks_output: :verbose"))
	if global.HooksOutputFor("pre-deploy") != "verbose" || global.HooksOutputFor("post-deploy") != "verbose" {
		t.Fatal("global hooks_output not applied")
	}
	perHook := mustBuild(t, newWorld(), merged(t, baseDeploy, "hooks_output:\n  pre-deploy: :verbose\n  post-deploy: :quiet"))
	if perHook.HooksOutputFor("pre-deploy") != "verbose" || perHook.HooksOutputFor("post-deploy") != "quiet" {
		t.Fatal("per-hook hooks_output not applied")
	}
	partial := mustBuild(t, newWorld(), merged(t, baseDeploy, "hooks_output:\n  pre-deploy: :verbose"))
	if partial.HooksOutputFor("post-deploy") != "" {
		t.Fatal("unconfigured hook has an output level")
	}
}

func TestHooksOutputInvalid(t *testing.T) {
	_, err := newWorld().build(merged(t, baseDeploy, "hooks_output: :invalid"))
	expectErrorContaining(t, err, "Invalid hooks_output 'invalid'")
	_, err = newWorld().build(merged(t, baseDeploy, "hooks_output:\n  pre-deploy: :invalid"))
	expectErrorContaining(t, err, "Invalid hooks_output 'invalid' for hook 'pre-deploy'")
}
