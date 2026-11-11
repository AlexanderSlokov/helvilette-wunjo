package configuration

// Ports test/configuration/accessory_test.rb at Kamal 2.12.0.

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/testsupport"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

const accessoryDeploy = `
service: app
image: dhh/app
registry: {username: dhh, password: secret}
servers:
  web: [{1.1.1.1: writer}, {1.1.1.2: reader}]
  workers: [{1.1.1.3: writer}, 1.1.1.4]
builder: {arch: amd64}
env: {REDIS_URL: redis://x/y}
accessories:
  mysql:
    image: public.registry/mysql:8.0
    host: 1.1.1.5
    port: "127.0.0.1:3306:3306"
    env:
      clear: {MYSQL_ROOT_HOST: "%"}
      secret: [MYSQL_ROOT_PASSWORD]
    files:
      - config/mysql/my.cnf:/etc/mysql/my.cnf
      - db/structure.sql:/docker-entrypoint-initdb.d/structure.sql
    directories:
      - data:/var/lib/mysql
  redis:
    image: redis:latest
    hosts: [1.1.1.6, 1.1.1.7]
    port: "127.0.0.1:6379:6379"
    labels: {cache: "true"}
    env: {SOMETHING: else}
    volumes: [/var/lib/redis:/data]
    options: {cpus: "4", memory: 2GB}
  monitoring:
    service: custom-monitoring
    image: monitoring:latest
    registry: {server: other.registry, username: user, password: pw}
    role: web
    port: "127.0.0.1:4321:4321"
    labels: {cache: "true"}
    env: {STATSD_PORT: "8126"}
    options: {cpus: "4", memory: 2GB}
    proxy: {host: monitoring.example.com}
  proxy:
    image: proxy:latest
    tags: [writer, reader]
  logger:
    image: logger:latest
    tag: writer
`

func accessoryRaw(t *testing.T) *yamlord.Ordered { return parseYAML(t, accessoryDeploy) }

func mysqlOf(raw *yamlord.Ordered) *yamlord.Ordered {
	return raw.At("accessories").Map().At("mysql").Map()
}

func setYAML(t *testing.T, m *yamlord.Ordered, key, value string) {
	t.Helper()
	m.Set(key, parseYAML(t, "v: "+value).At("v"))
}

func accessoryConfig(t *testing.T) *Configuration {
	return mustBuild(t, newWorld(), accessoryRaw(t))
}

func TestAccessoryServiceName(t *testing.T) {
	cfg := accessoryConfig(t)
	if cfg.Accessory("mysql").ServiceName() != "app-mysql" || cfg.Accessory("redis").ServiceName() != "app-redis" || cfg.Accessory("monitoring").ServiceName() != "custom-monitoring" {
		t.Fatal("wrong service names")
	}
}

func TestAccessoryImage(t *testing.T) {
	cfg := accessoryConfig(t)
	if cfg.Accessory("mysql").Image() != "public.registry/mysql:8.0" || cfg.Accessory("redis").Image() != "redis:latest" || cfg.Accessory("monitoring").Image() != "other.registry/monitoring:latest" {
		t.Fatal("wrong images")
	}
}

func TestAccessoryRegistry(t *testing.T) {
	cfg := accessoryConfig(t)
	if cfg.Accessory("mysql").Registry() != nil || cfg.Accessory("redis").Registry() != nil {
		t.Fatal("unexpected accessory registry")
	}
	registry := cfg.Accessory("monitoring").Registry()
	user, _ := registry.Username()
	password, _ := registry.Password()
	if registry.Server() != "other.registry" || user != "user" || password != "pw" {
		t.Fatal("wrong monitoring registry")
	}
}

func TestAccessoryPort(t *testing.T) {
	cfg := accessoryConfig(t)
	if cfg.Accessory("mysql").Port() != "127.0.0.1:3306:3306" || cfg.Accessory("redis").Port() != "127.0.0.1:6379:6379" {
		t.Fatal("wrong ports")
	}
	raw := accessoryRaw(t)
	mysqlOf(raw).Delete("port")
	if mustBuild(t, newWorld(), raw).Accessory("mysql").Port() != "" {
		t.Fatal("port set without port config")
	}
}

func TestExplicitBindAddressesArePreservedVerbatim(t *testing.T) {
	for _, port := range []string{"127.0.0.1:3306:3306", "0.0.0.0:3306:3306", "192.168.1.1:3306:3306", "[::1]:3306:3306", "[::]::3306", "192.168.1.1:3306:3306/tcp", "127.0.0.1:3306:3306/udp"} {
		raw := accessoryRaw(t)
		mysqlOf(raw).Set("port", yamlord.NewString(port))
		if got := mustBuild(t, newWorld(), raw).Accessory("mysql").Port(); got != port {
			t.Errorf("port %q came back as %q", port, got)
		}
	}
}

func TestPublishingThroughOptionsRequiresABindAddress(t *testing.T) {
	raw := accessoryRaw(t)
	mysqlOf(raw).Delete("port")
	setYAML(t, mysqlOf(raw), "options", `{publish: "3306:3306"}`)
	_, err := newWorld().build(raw)
	expectErrorContaining(t, err, `options/publish "3306:3306" must name a bind address`)
}

func TestABoundPublishInOptionsIsAccepted(t *testing.T) {
	raw := accessoryRaw(t)
	mysqlOf(raw).Delete("port")
	setYAML(t, mysqlOf(raw), "options", `{publish: ["127.0.0.1:3306:3306"]}`)
	if got := mustBuild(t, newWorld(), raw).Accessory("mysql").OptionArgs().String(); got != `--publish "127.0.0.1:3306:3306"` {
		t.Fatalf("option args = %s", got)
	}
}

func TestABlankPortPublishesNothing(t *testing.T) {
	raw := accessoryRaw(t)
	mysqlOf(raw).Set("port", yamlord.NewString(""))
	if mustBuild(t, newWorld(), raw).Accessory("mysql").PublishArgs() != nil {
		t.Fatal("blank port published")
	}
}

func TestPortWithoutBindAddressRaisesAnError(t *testing.T) {
	for _, port := range []string{"3306", "3306:3306", "3306:3306/udp", "127.0.0.1:3306", "999.999.999.999:3306:3306", "[not::valid::ip]:3306:3306", "[::1:3306:3306"} {
		raw := accessoryRaw(t)
		mysqlOf(raw).Set("port", yamlord.NewString(port))
		_, err := newWorld().build(raw)
		assertConfigurationError(t, err)
		if !strings.Contains(err.Error(), "must name a bind address") {
			t.Errorf("port %q: %v", port, err)
		}
	}
}

func TestAccessoryHosts(t *testing.T) {
	cfg := accessoryConfig(t)
	want := map[string]string{
		"mysql": "1.1.1.5", "redis": "1.1.1.6 1.1.1.7", "monitoring": "1.1.1.1 1.1.1.2",
		"proxy": "1.1.1.1 1.1.1.3 1.1.1.2", "logger": "1.1.1.1 1.1.1.3",
	}
	for name, hosts := range want {
		if got := joined(cfg.Accessory(name).Hosts()); got != hosts {
			t.Errorf("%s hosts = %s, want %s", name, got, hosts)
		}
	}
}

func TestMissingHost(t *testing.T) {
	raw := accessoryRaw(t)
	mysqlOf(raw).Set("host", yamlord.NewNull())
	_, err := newWorld().build(raw)
	assertConfigurationError(t, err)
}

func TestSettingHostHostsRolesAndTags(t *testing.T) {
	raw := accessoryRaw(t)
	setYAML(t, mysqlOf(raw), "hosts", "[mysql-db1]")
	setYAML(t, mysqlOf(raw), "roles", "[db]")
	_, err := newWorld().build(raw)
	expectError(t, err, "accessories/mysql: specify one of `host`, `hosts`, `role`, `roles`, `tag` or `tags`")
}

func TestAccessoryAllHosts(t *testing.T) {
	if got := joined(accessoryConfig(t).AllHosts()); got != "1.1.1.1 1.1.1.2 1.1.1.3 1.1.1.4 1.1.1.5 1.1.1.6 1.1.1.7" {
		t.Fatalf("all hosts = %s", got)
	}
}

func TestAccessoryLabelArgs(t *testing.T) {
	cfg := accessoryConfig(t)
	if cfg.Accessory("mysql").LabelArgs().String() != `--label service="app-mysql"` || cfg.Accessory("redis").LabelArgs().String() != `--label service="app-redis" --label cache="true"` {
		t.Fatal("wrong label args")
	}
}

func TestAccessoryEnvArgs(t *testing.T) {
	cfg := mustBuild(t, newWorld().withSecrets("MYSQL_ROOT_PASSWORD=secret123"), accessoryRaw(t))
	mysql, redis := cfg.Accessory("mysql"), cfg.Accessory("redis")
	if got := mysql.EnvArgs().String(); got != `--env MYSQL_ROOT_HOST="%" --env-file .kamal/apps/app/env/accessories/mysql.env` {
		t.Fatalf("mysql env args = %s", got)
	}
	if file, err := mysql.SecretsFile(); err != nil || file != "MYSQL_ROOT_PASSWORD=secret123\n" {
		t.Fatalf("mysql secrets = %q, %v", file, err)
	}
	if got := redis.EnvArgs().String(); got != `--env SOMETHING="else" --env-file .kamal/apps/app/env/accessories/redis.env` {
		t.Fatalf("redis env args = %s", got)
	}
	if file, _ := redis.SecretsFile(); file != "\n" {
		t.Fatalf("redis secrets = %q", file)
	}
}

func volumeArgs(t *testing.T, a *Accessory) string {
	t.Helper()
	line, err := a.VolumeArgs()
	if err != nil {
		t.Fatal(err)
	}
	return line.String()
}

func TestAccessoryVolumeArgs(t *testing.T) {
	cfg := accessoryConfig(t)
	want := "--volume $PWD/app-mysql/etc/mysql/my.cnf:/etc/mysql/my.cnf --volume $PWD/app-mysql/docker-entrypoint-initdb.d/structure.sql:/docker-entrypoint-initdb.d/structure.sql --volume $PWD/app-mysql/data:/var/lib/mysql"
	if got := volumeArgs(t, cfg.Accessory("mysql")); got != want {
		t.Fatalf("mysql volume args = %s", got)
	}
	if got := volumeArgs(t, cfg.Accessory("redis")); got != "--volume /var/lib/redis:/data" {
		t.Fatalf("redis volume args = %s", got)
	}
	raw := accessoryRaw(t)
	setYAML(t, raw.At("accessories").Map().At("redis").Map(), "volumes", "[redis_data:/data]")
	if got := volumeArgs(t, mustBuild(t, newWorld(), raw).Accessory("redis")); got != "--volume redis_data:/data" {
		t.Fatalf("named volume args = %s", got)
	}
}

// erbWorld stores the structure.sql.erb fixture at the path the Ruby test uses.
func erbWorld(t *testing.T, secretsFile string) *fakeWorld {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(testsupport.FixturesDir(), "files", "structure.sql.erb"))
	if err != nil {
		t.Fatal(err)
	}
	w := newWorld().withSecrets(secretsFile)
	w.files["test/fixtures/files/structure.sql.erb"] = string(content)
	return w
}

func withERBFile(t *testing.T, raw *yamlord.Ordered, replaceFiles bool) {
	t.Helper()
	mysql := mysqlOf(raw)
	erbFile := yamlord.NewString("test/fixtures/files/structure.sql.erb:/docker-entrypoint-initdb.d/structure.sql")
	files := []*yamlord.Value{erbFile}
	if !replaceFiles {
		files = append(mysql.At("files").List(), erbFile)
	}
	mysql.Set("files", yamlord.NewList(files...))
	env := mysql.At("env").Map()
	env.Set("secret", yamlord.NewList(append(env.At("secret").List(), yamlord.NewString("ENV_VAR:SECRET_VAR"))...))
}

func TestDynamicFileExpansion(t *testing.T) {
	raw := accessoryRaw(t)
	withERBFile(t, raw, false)
	cfg := mustBuild(t, erbWorld(t, "MYSQL_ROOT_PASSWORD=secret123\nSECRET_VAR=secret_env_value"), raw)
	files, err := cfg.Accessory("mysql").Files()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"This was dynamically expanded", "%", "secret123", "secret_env_value"} {
		if !strings.Contains(files[2].Content, want) {
			t.Errorf("rendered file lacks %q:\n%s", want, files[2].Content)
		}
	}
}

func TestParallelERBFileRendersKeepAccessoryEnvInterpolations(t *testing.T) {
	raw := accessoryRaw(t)
	withERBFile(t, raw, true)
	accessory := mustBuild(t, erbWorld(t, "MYSQL_ROOT_PASSWORD=secret123\nSECRET_VAR=secret_env_value"), raw).Accessory("mysql")
	renders := make([]string, 2)
	var wg sync.WaitGroup
	for i := range renders {
		wg.Go(func() {
			files, err := accessory.Files()
			if err == nil {
				renders[i] = files[0].Content
			}
		})
	}
	wg.Wait()
	for i, contents := range renders {
		for _, want := range []string{"secret123", "secret_env_value", "%"} {
			if !strings.Contains(contents, want) {
				t.Errorf("render %d lost %q", i, want)
			}
		}
	}
}

func directoriesOf(t *testing.T, directories string, files string) []AccessoryPath {
	t.Helper()
	raw := accessoryRaw(t)
	setYAML(t, mysqlOf(raw), "directories", directories)
	if files != "" {
		setYAML(t, mysqlOf(raw), "files", files)
	}
	return mustBuild(t, newWorld(), raw).Accessory("mysql").Directories()
}

func TestDirectoryPaths(t *testing.T) {
	cases := []struct {
		directories string
		want        AccessoryPath
	}{
		{"[data:/var/lib/mysql]", AccessoryPath{Local: "$PWD/app-mysql/data", HostPath: "app-mysql/data", ContainerPath: "/var/lib/mysql"}},
		{"[/var/data/mysql:/var/lib/mysql]", AccessoryPath{Local: "/var/data/mysql", HostPath: "/var/data/mysql", ContainerPath: "/var/lib/mysql"}},
		{"[data:/var/lib/mysql:z]", AccessoryPath{Local: "$PWD/app-mysql/data", HostPath: "app-mysql/data", ContainerPath: "/var/lib/mysql", Options: "z"}},
		{"[{local: data, remote: /var/lib/mysql, mode: '0750'}]", AccessoryPath{Local: "$PWD/app-mysql/data", HostPath: "app-mysql/data", ContainerPath: "/var/lib/mysql", Mode: "0750"}},
		{"[{local: data, remote: /var/lib/mysql, owner: 'mysql:mysql'}]", AccessoryPath{Local: "$PWD/app-mysql/data", HostPath: "app-mysql/data", ContainerPath: "/var/lib/mysql", Owner: "mysql:mysql"}},
		{"[{local: data, remote: /var/lib/mysql, mode: '0750', owner: '1000:1000', options: z}]", AccessoryPath{Local: "$PWD/app-mysql/data", HostPath: "app-mysql/data", ContainerPath: "/var/lib/mysql", Options: "z", Mode: "0750", Owner: "1000:1000"}},
	}
	for _, c := range cases {
		got := directoriesOf(t, c.directories, "[]")
		if len(got) != 1 || got[0] != c.want {
			t.Errorf("%s: got %+v, want %+v", c.directories, got, c.want)
		}
	}
}

func TestDirectoryWithMountOptionsVolumeArgs(t *testing.T) {
	raw := accessoryRaw(t)
	setYAML(t, mysqlOf(raw), "files", "[]")
	setYAML(t, mysqlOf(raw), "directories", "[data:/var/lib/mysql:z]")
	if got := volumeArgs(t, mustBuild(t, newWorld(), raw).Accessory("mysql")); got != "--volume $PWD/app-mysql/data:/var/lib/mysql:z" {
		t.Fatalf("volume args = %s", got)
	}
}

func filesOf(t *testing.T, w *fakeWorld, files string) (*Accessory, []AccessoryPath) {
	t.Helper()
	raw := accessoryRaw(t)
	setYAML(t, mysqlOf(raw), "files", files)
	setYAML(t, mysqlOf(raw), "directories", "[]")
	accessory := mustBuild(t, w, raw).Accessory("mysql")
	paths, err := accessory.Files()
	if err != nil {
		t.Fatal(err)
	}
	return accessory, paths
}

func TestFileSettings(t *testing.T) {
	cases := []struct {
		files                string
		options, mode, owner string
		volume               string
	}{
		{`["config/mysql/my.cnf:/etc/mysql/my.cnf:ro,z"]`, "ro,z", "755", "", "--volume $PWD/app-mysql/etc/mysql/my.cnf:/etc/mysql/my.cnf:ro,z"},
		{"[config/mysql/my.cnf:/etc/mysql/my.cnf]", "", "755", "", ""},
		{"[{local: config/mysql/my.cnf, remote: /etc/mysql/my.cnf, mode: '0600'}]", "", "0600", "", ""},
		{"[{local: config/mysql/my.cnf, remote: /etc/mysql/my.cnf, owner: 'mysql:mysql'}]", "", "755", "mysql:mysql", ""},
		{"[{local: config/mysql/my.cnf, remote: /etc/mysql/my.cnf, mode: '0640', owner: '1000:1000', options: Z}]", "Z", "0640", "1000:1000", "--volume $PWD/app-mysql/etc/mysql/my.cnf:/etc/mysql/my.cnf:Z"},
	}
	for _, c := range cases {
		accessory, paths := filesOf(t, newWorld(), c.files)
		got := paths[0]
		if got.Options != c.options || got.Mode != c.mode || got.Owner != c.owner || got.Local != "/home/app/project/config/mysql/my.cnf" {
			t.Errorf("%s: got %+v", c.files, got)
		}
		if c.volume != "" && volumeArgs(t, accessory) != c.volume {
			t.Errorf("%s: volume args = %s", c.files, volumeArgs(t, accessory))
		}
	}
}

func TestFileWithHashFormatERBExpansion(t *testing.T) {
	_, paths := filesOf(t, erbWorld(t, "MYSQL_ROOT_PASSWORD=secret123"), "[{local: test/fixtures/files/structure.sql.erb, remote: /docker-entrypoint-initdb.d/structure.sql}]")
	if !paths[0].Rendered || !strings.Contains(paths[0].Content, "This was dynamically expanded") {
		t.Fatalf("file = %+v", paths[0])
	}
}

func TestAccessoryOptionsAndRestartPolicy(t *testing.T) {
	cfg := accessoryConfig(t)
	if cfg.Accessory("redis").OptionArgs().String() != `--cpus "4" --memory "2GB"` || cfg.Accessory("redis").RestartPolicy() != "unless-stopped" {
		t.Fatal("wrong redis options")
	}
	raw := accessoryRaw(t)
	raw.At("accessories").Map().At("redis").Map().At("options").Map().Set("restart", yamlord.NewString("always"))
	redis := mustBuild(t, newWorld(), raw).Accessory("redis")
	if redis.RestartPolicy() != "always" || redis.OptionArgs().String() != `--cpus "4" --memory "2GB"` {
		t.Fatal("restart option not separated")
	}
}

func TestAccessoryNetworkArgs(t *testing.T) {
	if got := accessoryConfig(t).Accessory("mysql").NetworkArgs().String(); got != "--network kamal" {
		t.Fatalf("network args = %s", got)
	}
	raw := accessoryRaw(t)
	mysqlOf(raw).Set("network", yamlord.NewString("database"))
	if got := mustBuild(t, newWorld(), raw).Accessory("mysql").NetworkArgs().String(); got != "--network database" {
		t.Fatalf("network args = %s", got)
	}
}

func TestAccessoryProxy(t *testing.T) {
	monitoring := accessoryConfig(t).Accessory("monitoring")
	if !monitoring.RunningProxy() || joined(monitoring.Proxy().Hosts()) != "monitoring.example.com" {
		t.Fatal("monitoring proxy not set")
	}
}

func TestAccessoryInvalidBooleanRestartPolicy(t *testing.T) {
	raw := accessoryRaw(t)
	setYAML(t, mysqlOf(raw), "options", "{restart: false}")
	_, err := newWorld().build(raw)
	expectError(t, err, `accessories/mysql/options/restart: should be a string. Use "no" to disable restarts`)
}
