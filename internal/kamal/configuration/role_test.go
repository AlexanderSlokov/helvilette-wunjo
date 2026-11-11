package configuration

// Ports test/configuration/role_test.rb at Kamal 2.12.0.

import (
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

const roleDeploy = `
service: app
image: dhh/app
registry: {username: dhh, password: secret}
servers: [1.1.1.1, 1.1.1.2]
builder: {arch: amd64}
env: {REDIS_URL: redis://x/y}
`

const roleDeployWithRoles = roleDeploy + `
servers:
  web: [1.1.1.1, 1.1.1.2]
  workers:
    hosts: [1.1.1.3, 1.1.1.4]
    cmd: bin/jobs
    env:
      REDIS_URL: redis://a/b
      WEB_CONCURRENCY: "4"
`

func withRolesConfig(t *testing.T, w *fakeWorld, overrides ...string) *Configuration {
	t.Helper()
	return mustBuild(t, w, deepMerged(t, roleDeployWithRoles, overrides...))
}

func lineOf(t *testing.T, line interface{ String() string }, err error) string {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	return line.String()
}

func TestRoleHosts(t *testing.T) {
	if got := joined(mustBuild(t, newWorld(), parseYAML(t, roleDeploy)).Role("web").Hosts()); got != "1.1.1.1 1.1.1.2" {
		t.Fatalf("hosts = %s", got)
	}
	if got := joined(withRolesConfig(t, newWorld()).Role("workers").Hosts()); got != "1.1.1.3 1.1.1.4" {
		t.Fatalf("hosts = %s", got)
	}
}

func TestMissingEnvTagIsIgnored(t *testing.T) {
	raw := deepMerged(t, roleDeployWithRoles)
	raw.At("servers").Map().At("workers").Map().Set("hosts", parseYAML(t, "h: [{1.1.1.3: [job]}]").At("h"))
	env, err := mustBuild(t, newWorld(), raw).Role("workers").Env("1.1.1.3")
	if err != nil || env.Clear().At("REDIS_URL").ToS() != "redis://a/b" {
		t.Fatalf("env = %v, %v", env, err)
	}
}

func TestRoleCmd(t *testing.T) {
	if !mustBuild(t, newWorld(), parseYAML(t, roleDeploy)).Role("web").Cmd().IsNull() {
		t.Fatal("web has a cmd")
	}
	if got := withRolesConfig(t, newWorld()).Role("workers").Cmd().ToS(); got != "bin/jobs" {
		t.Fatalf("cmd = %q", got)
	}
}

func TestLabelArgs(t *testing.T) {
	if got := withRolesConfig(t, newWorld()).Role("workers").LabelArgs().String(); got != `--label service="app" --label role="workers" --label destination` {
		t.Fatalf("label args = %s", got)
	}
	if got := mustBuild(t, newWorld(), parseYAML(t, roleDeploy)).Role("web").LabelArgs().String(); got != `--label service="app" --label role="web" --label destination` {
		t.Fatalf("label args = %s", got)
	}
}

func TestCustomLabels(t *testing.T) {
	cfg := mustBuild(t, newWorld(), merged(t, roleDeploy, `labels: {my.custom.label: "50"}`))
	if got := cfg.Role("web").Labels().At("my.custom.label").ToS(); got != "50" {
		t.Fatalf("label = %q", got)
	}
	special := withRolesConfig(t, newWorld(), `labels: {my.custom.label: "50"}`, `servers: {workers: {labels: {my.custom.label: "70"}}}`)
	if got := special.Role("workers").Labels().At("my.custom.label").ToS(); got != "70" {
		t.Fatalf("label = %q", got)
	}
}

func TestDefaultProxyLabelOnNonWebRole(t *testing.T) {
	cfg := withRolesConfig(t, newWorld(), "servers: {beta: {proxy: true, hosts: [1.1.1.5]}}")
	if got := cfg.Role("beta").LabelArgs().String(); got != `--label service="app" --label role="beta" --label destination` {
		t.Fatalf("label args = %s", got)
	}
}

func assertWorkerEnv(t *testing.T, cfg *Configuration, wantArgs, wantSecrets string) {
	t.Helper()
	role := cfg.Role("workers")
	args, err := role.EnvArgs("1.1.1.3")
	if got := lineOf(t, args, err); got != wantArgs {
		t.Fatalf("env args = %s\nwant       %s", got, wantArgs)
	}
	secretsFile, err := role.SecretsFile("1.1.1.3")
	if err != nil || secretsFile != wantSecrets {
		t.Fatalf("secrets file = %q, %v; want %q", secretsFile, err, wantSecrets)
	}
}

const workerArgs = `--env REDIS_URL="redis://a/b" --env WEB_CONCURRENCY="4" --env-file .kamal/apps/app/env/roles/workers.env`

func TestEnvOverwrittenByRole(t *testing.T) {
	cfg := withRolesConfig(t, newWorld())
	env, _ := cfg.Role("workers").Env("1.1.1.3")
	if env.Clear().At("REDIS_URL").ToS() != "redis://a/b" {
		t.Fatal("role env did not override")
	}
	assertWorkerEnv(t, cfg, workerArgs, "\n")
}

func TestRoleContainerName(t *testing.T) {
	w := newWorld()
	w.env["VERSION"] = "12345"
	cfg := withRolesConfig(t, w)
	if cfg.Role("workers").ContainerName("") != "app-workers-12345" || cfg.Role("web").ContainerName("") != "app-web-12345" {
		t.Fatal("wrong container names")
	}
}

func TestEnvSecretOverwrittenByRole(t *testing.T) {
	w := newWorld().withSecrets("REDIS_PASSWORD=secret456\nDB_PASSWORD=secret&\"123")
	raw := merged(t, roleDeployWithRoles, "env: {clear: {REDIS_URL: redis://a/b}, secret: [REDIS_PASSWORD]}")
	raw.At("servers").Map().At("workers").Map().Set("env", parseYAML(t, `e: {clear: {REDIS_URL: redis://a/b, WEB_CONCURRENCY: "4"}, secret: [DB_PASSWORD]}`).At("e"))
	assertWorkerEnv(t, mustBuild(t, w, raw), workerArgs, "REDIS_PASSWORD=secret456\nDB_PASSWORD=secret&\"123\n")
}

func TestEnvSecretsOnlyInRole(t *testing.T) {
	w := newWorld().withSecrets("DB_PASSWORD=secret123")
	raw := deepMerged(t, roleDeployWithRoles)
	raw.At("servers").Map().At("workers").Map().Set("env", parseYAML(t, `e: {clear: {REDIS_URL: redis://a/b, WEB_CONCURRENCY: "4"}, secret: [DB_PASSWORD]}`).At("e"))
	assertWorkerEnv(t, mustBuild(t, w, raw), workerArgs, "DB_PASSWORD=secret123\n")
}

func TestEnvSecretsOnlyAtTopLevel(t *testing.T) {
	w := newWorld().withSecrets("REDIS_PASSWORD=secret456")
	cfg := mustBuild(t, w, merged(t, roleDeployWithRoles, "env: {clear: {REDIS_URL: redis://a/b}, secret: [REDIS_PASSWORD]}"))
	assertWorkerEnv(t, cfg, workerArgs, "REDIS_PASSWORD=secret456\n")
}

func TestEnvOverwrittenByRoleWithSecrets(t *testing.T) {
	w := newWorld().withSecrets("REDIS_PASSWORD=secret456")
	raw := merged(t, roleDeployWithRoles, "env: {clear: {REDIS_URL: redis://a/b}, secret: [REDIS_PASSWORD]}")
	raw.At("servers").Map().At("workers").Map().Set("env", parseYAML(t, "e: {clear: {REDIS_URL: redis://c/d}}").At("e"))
	assertWorkerEnv(t, mustBuild(t, w, raw), `--env REDIS_URL="redis://c/d" --env-file .kamal/apps/app/env/roles/workers.env`, "REDIS_PASSWORD=secret456\n")
}

func versionWorld() *fakeWorld {
	w := newWorld()
	w.env["VERSION"] = "12345"
	return w
}

func TestAssetPathAndVolumeArgs(t *testing.T) {
	plain := withRolesConfig(t, versionWorld())
	for _, name := range []string{"web", "workers"} {
		role := plain.Role(name)
		if role.AssetVolumeArgs() != nil || role.AssetPath() != "" || role.Assets() {
			t.Fatalf("%s has assets", name)
		}
	}
	rootAssets := withRolesConfig(t, versionWorld(), "asset_path: foo")
	assertAssets(t, rootAssets, "foo", "foo", "--volume $PWD/.kamal/apps/app/assets/volumes/web-12345:foo")

	raw := deepMerged(t, roleDeployWithRoles)
	replaceRole(t, raw, "web", "{hosts: [1.1.1.1, 1.1.1.2], asset_path: bar}")
	assertAssets(t, mustBuild(t, versionWorld(), raw), "bar", "", "--volume $PWD/.kamal/apps/app/assets/volumes/web-12345:bar")
}

func assertAssets(t *testing.T, cfg *Configuration, webPath, workersPath, webArgs string) {
	t.Helper()
	web, workers := cfg.Role("web"), cfg.Role("workers")
	if web.AssetPath() != webPath || workers.AssetPath() != workersPath {
		t.Fatalf("asset paths %q %q", web.AssetPath(), workers.AssetPath())
	}
	if web.AssetVolumeArgs().String() != webArgs || workers.AssetVolumeArgs() != nil {
		t.Fatalf("asset volume args %q %v", web.AssetVolumeArgs().String(), workers.AssetVolumeArgs())
	}
	if !web.Assets() || workers.Assets() {
		t.Fatal("assets? is wrong")
	}
}

func TestAssetPathWithMountOptions(t *testing.T) {
	root := withRolesConfig(t, versionWorld(), `asset_path: "/rails/public/assets:z"`)
	web := root.Role("web")
	if web.AssetPath() != "/rails/public/assets" || web.AssetPathOptions() != "z" || web.AssetVolumeArgs().String() != "--volume $PWD/.kamal/apps/app/assets/volumes/web-12345:/rails/public/assets:z" {
		t.Fatalf("root asset path: %q %q %q", web.AssetPath(), web.AssetPathOptions(), web.AssetVolumeArgs().String())
	}
	raw := deepMerged(t, roleDeployWithRoles)
	replaceRole(t, raw, "web", `{hosts: [1.1.1.1, 1.1.1.2], asset_path: "/assets:ro,z"}`)
	web = mustBuild(t, versionWorld(), raw).Role("web")
	if web.AssetPath() != "/assets" || web.AssetPathOptions() != "ro,z" || web.AssetVolumeArgs().String() != "--volume $PWD/.kamal/apps/app/assets/volumes/web-12345:/assets:ro,z" {
		t.Fatalf("role asset path: %q %q %q", web.AssetPath(), web.AssetPathOptions(), web.AssetVolumeArgs().String())
	}
}

func TestAssetDirectories(t *testing.T) {
	cfg := withRolesConfig(t, versionWorld())
	if cfg.Role("web").AssetExtractedDirectory("12345") != ".kamal/apps/app/assets/extracted/web-12345" ||
		cfg.Role("workers").AssetExtractedDirectory("12345") != ".kamal/apps/app/assets/extracted/workers-12345" {
		t.Fatal("wrong extracted directories")
	}
	if cfg.Role("web").AssetVolumeDirectory("12345") != ".kamal/apps/app/assets/volumes/web-12345" ||
		cfg.Role("workers").AssetVolumeDirectory("12345") != ".kamal/apps/app/assets/volumes/workers-12345" {
		t.Fatal("wrong volume directories")
	}
}

// replaceRole sets servers[name] to the YAML value, as the Ruby tests assign
// @deploy_with_roles[:servers][name].
func replaceRole(t *testing.T, raw *yamlord.Ordered, name, value string) {
	t.Helper()
	raw.At("servers").Map().Set(name, parseYAML(t, "v: "+value).At("v"))
}

func TestStopArgs(t *testing.T) {
	webTimeout := func(raw *yamlord.Ordered) {
		replaceRole(t, raw, "web", "{hosts: [1.1.1.1, 1.1.1.2], stop_timeout: 60}")
	}
	workersTimeout := func(raw *yamlord.Ordered) {
		replaceRole(t, raw, "workers", "{hosts: [1.1.1.3, 1.1.1.4], cmd: bin/jobs, stop_timeout: 60}")
	}
	rootTimeout := func(raw *yamlord.Ordered) { raw.Set("stop_timeout", yamlord.NewInt(45)) }
	cases := []struct {
		name      string
		edits     []func(*yamlord.Ordered)
		web, work string
	}{
		{"with proxy", nil, "", "-t 30"},
		{"web stop_timeout", []func(*yamlord.Ordered){webTimeout}, "-t 60", "-t 30"},
		{"workers stop_timeout", []func(*yamlord.Ordered){workersTimeout}, "", "-t 60"},
		{"root stop_timeout", []func(*yamlord.Ordered){rootTimeout}, "-t 45", "-t 45"},
		{"role overrides root", []func(*yamlord.Ordered){rootTimeout, webTimeout}, "-t 60", "-t 45"},
	}
	for _, c := range cases {
		raw := deepMerged(t, roleDeployWithRoles)
		for _, edit := range c.edits {
			edit(raw)
		}
		cfg := mustBuild(t, newWorld(), raw)
		if got := cfg.Role("web").StopArgs().String(); got != c.web {
			t.Errorf("%s: web stop args %q, want %q", c.name, got, c.web)
		}
		if got := cfg.Role("workers").StopArgs().String(); got != c.work {
			t.Errorf("%s: workers stop args %q, want %q", c.name, got, c.work)
		}
	}
}

func TestRestartPolicy(t *testing.T) {
	cfg := withRolesConfig(t, newWorld(), `servers: {workers: {options: {restart: "on-failure:3", memory: 2g}}}`)
	role := cfg.Role("workers")
	if role.RestartPolicy() != "on-failure:3" || role.OptionArgs().String() != `--memory "2g"` {
		t.Fatalf("restart %q options %q", role.RestartPolicy(), role.OptionArgs().String())
	}
	if withRolesConfig(t, newWorld()).Role("workers").RestartPolicy() != "unless-stopped" {
		t.Fatal("wrong default restart policy")
	}
}

func TestRoleSpecificProxyConfig(t *testing.T) {
	cfg := withRolesConfig(t, newWorld(), "proxy: {response_timeout: 15}", "servers: {workers: {proxy: {response_timeout: 18}}}")
	if got := deployOption(cfg.Role("web").Proxy().DeployOptions(), "target-timeout"); got != "15s" {
		t.Fatalf("web target timeout %q", got)
	}
	if got := deployOption(cfg.Role("workers").Proxy().DeployOptions(), "target-timeout"); got != "18s" {
		t.Fatalf("workers target timeout %q", got)
	}
}

func deployOption(attrs []utils.Attr, key string) string {
	for _, attr := range attrs {
		if attr.Key == key {
			return attr.Value.ToS()
		}
	}
	return ""
}

func TestInvalidBooleanRestartPolicy(t *testing.T) {
	_, err := newWorld().build(deepMerged(t, roleDeployWithRoles, "servers: {workers: {options: {restart: false}}}"))
	expectError(t, err, `servers/workers/options/restart: should be a string. Use "no" to disable restarts`)
}
