package configuration

// Ports test/configuration/boot_test.rb, env_test.rb, env/tags_test.rb,
// output_test.rb, proxy_test.rb, proxy/boot_test.rb, proxy/run_test.rb,
// ssh_test.rb, sshkit_test.rb, and volume_test.rb at Kamal 2.12.0.

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/secrets"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/testsupport"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

const shortDeploy = `
service: app
image: dhh/app
registry: {username: dhh, password: secret}
builder: {arch: amd64}
servers: [1.1.1.1]
`

func bootConfig(t *testing.T, boot string) *Boot {
	deploy := "service: app\nimage: dhh/app\nregistry: {username: dhh, password: secret}\nbuilder: {arch: amd64}\nservers: {web: [1.1.1.1, 1.1.1.2], workers: [1.1.1.3, 1.1.1.4]}\n"
	if boot != "" {
		deploy += "boot: " + boot + "\n"
	}
	return mustBuild(t, newWorld(), parseYAML(t, deploy)).Boot()
}

func TestBoot(t *testing.T) {
	none := bootConfig(t, "")
	if !none.Limit().IsNull() || !none.Wait().IsNull() || !none.ParallelRoles().IsNull() {
		t.Fatal("boot settings without boot config")
	}
	cases := map[string]int64{"{limit: 3, wait: 2}": 3, `{limit: "50%", wait: 2}`: 2, `{limit: "1%", wait: 2}`: 1}
	for boot, limit := range cases {
		b := bootConfig(t, boot)
		if b.Limit().Int() != limit || b.Wait().Int() != 2 {
			t.Errorf("%s: limit %s wait %s", boot, b.Limit().Inspect(), b.Wait().Inspect())
		}
	}
	if !bootConfig(t, "{parallel_roles: true}").ParallelRoles().IsTrue() {
		t.Fatal("parallel_roles not read")
	}
}

func assertEnv(t *testing.T, w *fakeWorld, config, wantArgs, wantSecrets string) {
	t.Helper()
	sec := secrets.New(secrets.Options{Files: w.files})
	env, err := newEnv(yamlord.NewMap(parseYAML(t, config)), sec, "env")
	if err != nil {
		t.Fatal(err)
	}
	if got := env.ClearArgs().String(); got != wantArgs {
		t.Fatalf("clear args = %s, want %s", got, wantArgs)
	}
	if got, err := env.SecretsFile(); err != nil || got != wantSecrets {
		t.Fatalf("secrets = %q, %v; want %q", got, err, wantSecrets)
	}
}

func TestEnvForms(t *testing.T) {
	both := `--env foo="bar" --env baz="haz"`
	assertEnv(t, newWorld(), "{foo: bar, baz: haz}", both, "\n")
	assertEnv(t, newWorld(), "{clear: {foo: bar, baz: haz}}", both, "\n")
	assertEnv(t, newWorld().withSecrets("PASSWORD=hello"), "{secret: [PASSWORD]}", "", "PASSWORD=hello\n")
	assertEnv(t, newWorld().withSecrets("PASSWORD=hello"), "{secret: [PASSWORD], clear: {foo: bar, baz: haz}}", both, "PASSWORD=hello\n")
	assertEnv(t, newWorld().withSecrets("ALIASED_PASSWORD=hello"), "{secret: ['PASSWORD:ALIASED_PASSWORD'], clear: {}}", "", "PASSWORD=hello\n")
}

func TestEnvMissingSecret(t *testing.T) {
	env, err := newEnv(yamlord.NewMap(parseYAML(t, "{secret: [PASSWORD]}")), secrets.New(secrets.Options{Files: testsupport.FakeFiles{}}), "env")
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.SecretsFile()
	assertConfigurationError(t, err)
}

const tagsDeploy = `
service: app
image: dhh/app
registry: {username: dhh, password: secret}
servers: [{1.1.1.1: odd}, {1.1.1.2: even}, {1.1.1.3: [odd, three]}]
builder: {arch: amd64}
env:
  clear: {REDIS_URL: redis://x/y, THREE: "false"}
  tags:
    odd: {TYPE: odd}
    even: {TYPE: even}
    three: {THREE: "true"}
`

func tagNames(cfg *Configuration) string {
	var names []string
	for _, tag := range cfg.EnvTags() {
		names = append(names, tag.Name)
	}
	return joined(names)
}

func tagClear(t *testing.T, cfg *Configuration, name string) string {
	t.Helper()
	env, err := cfg.EnvTag(name).Env()
	if err != nil {
		t.Fatal(err)
	}
	return yamlord.NewMap(env.Clear()).Inspect()
}

func TestEnvTags(t *testing.T) {
	cfg := mustBuild(t, newWorld(), parseYAML(t, tagsDeploy))
	if tagNames(cfg) != "odd even three" || tagClear(t, cfg, "odd") != `{"TYPE" => "odd"}` || tagClear(t, cfg, "three") != `{"THREE" => "true"}` {
		t.Fatal("wrong env tags")
	}
	withRoles := mustBuild(t, newWorld(), merged(t, tagsDeploy, `
servers:
  web: [{1.1.1.1: odd}, 1.1.1.2]
  workers:
    hosts: [{1.1.1.3: [odd, oddjob]}, 1.1.1.4]
    cmd: bin/jobs
    env: {REDIS_URL: redis://a/b, WEB_CONCURRENCY: 4}
env:
  tags:
    odd: {TYPE: odd}
    oddjob: {TYPE: oddjob}
`))
	if tagNames(withRoles) != "odd oddjob" || tagClear(t, withRoles, "oddjob") != `{"TYPE" => "oddjob"}` {
		t.Fatal("wrong env tags with roles")
	}
}

func hostEnv(t *testing.T, cfg *Configuration, host string) *Env {
	t.Helper()
	env, err := cfg.Role("web").Env(host)
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func TestTagOverridesEnv(t *testing.T) {
	cfg := mustBuild(t, newWorld(), parseYAML(t, tagsDeploy))
	if hostEnv(t, cfg, "1.1.1.1").Clear().At("THREE").ToS() != "false" || hostEnv(t, cfg, "1.1.1.3").Clear().At("THREE").ToS() != "true" {
		t.Fatal("tag did not override env")
	}
}

func tagDeploy(server, tags string) string {
	return "service: app\nimage: dhh/app\nregistry: {username: dhh, password: secret}\nservers: [" + server + "]\nbuilder: {arch: amd64}\nenv: {tags: " + tags + "}\n"
}

func TestLaterTagWins(t *testing.T) {
	cfg := mustBuild(t, newWorld(), parseYAML(t, tagDeploy("{1.1.1.1: [first, second]}", "{first: {TYPE: first}, second: {TYPE: second}}")))
	if hostEnv(t, cfg, "1.1.1.1").Clear().At("TYPE").ToS() != "second" {
		t.Fatal("later tag did not win")
	}
}

func TestTagSecretEnv(t *testing.T) {
	cases := map[string][2]string{
		"[PASSWORD]":                    {"PASSWORD=hello", "PASSWORD=hello\n"},
		"['PASSWORD:ALIASED_PASSWORD']": {"PASSWORD=hello\nALIASED_PASSWORD=aliased_hello", "PASSWORD=aliased_hello\n"},
	}
	for secret, c := range cases {
		cfg := mustBuild(t, newWorld().withSecrets(c[0]), parseYAML(t, tagDeploy("{1.1.1.1: secrets}", "{secrets: {secret: "+secret+"}}")))
		if got, err := hostEnv(t, cfg, "1.1.1.1").SecretsFile(); err != nil || got != c[1] {
			t.Errorf("%s: secrets = %q, %v", secret, got, err)
		}
	}
}

func TestTagClearEnv(t *testing.T) {
	cfg := mustBuild(t, newWorld(), parseYAML(t, tagDeploy("{1.1.1.1: clearly}", "{clearly: {clear: {FOO: bar}}}")))
	if hostEnv(t, cfg, "1.1.1.1").Clear().At("FOO").ToS() != "bar" {
		t.Fatal("tag clear env not applied")
	}
}

func TestOutput(t *testing.T) {
	if mustBuild(t, newWorld(), parseYAML(t, shortDeploy)).Output().Enabled() {
		t.Fatal("output enabled by default")
	}
	cases := map[string]string{
		"output: {otel: {endpoint: 'http://otel-gateway:4318'}}":                                "otel",
		"output: {file: {path: /var/log/kamal/}}":                                               "file",
		"output: {otel: {endpoint: 'http://otel-gateway:4318'}, file: {path: /var/log/kamal/}}": "otel file",
	}
	for output, kinds := range cases {
		o := mustBuild(t, newWorld(), merged(t, shortDeploy, output)).Output()
		if !o.Enabled() || joined(o.Kinds()) != kinds {
			t.Errorf("%s: enabled %v kinds %v", output, o.Enabled(), o.Kinds())
		}
	}
	if mustBuild(t, newWorld(), merged(t, shortDeploy, "output: {}")).Output().Enabled() {
		t.Fatal("empty output enabled")
	}
}

func TestOutputRequiredSettings(t *testing.T) {
	if _, err := newWorld().build(merged(t, shortDeploy, "output: {otel: {}}")); !errors.Is(err, ErrOtelEndpointRequired) {
		t.Fatalf("otel error = %v", err)
	}
	if _, err := newWorld().build(merged(t, shortDeploy, "output: {file: {}}")); !errors.Is(err, ErrFilePathRequired) {
		t.Fatalf("file error = %v", err)
	}
}

func proxyConfig(t *testing.T, w *fakeWorld, proxy string) (*Configuration, error) {
	return w.build(merged(t, shortDeploy, "proxy: "+proxy))
}

func TestProxySSL(t *testing.T) {
	for _, proxy := range []string{"{ssl: true, host: example.com}", "{ssl: true, host: 'example.com,anotherexample.com'}", "{ssl: true, hosts: [example.com, anotherexample.com]}"} {
		cfg, err := proxyConfig(t, newWorld(), proxy)
		if err != nil || !cfg.Proxy().SSL() {
			t.Errorf("%s: %v", proxy, err)
		}
	}
	for _, proxy := range []string{"{ssl: true}", "{ssl: true, host: example.com, hosts: [anotherexample.com]}", "false"} {
		_, err := proxyConfig(t, newWorld(), proxy)
		assertConfigurationError(t, err)
	}
	if cfg, _ := proxyConfig(t, newWorld(), "{ssl: false}"); cfg.Proxy().SSL() {
		t.Fatal("ssl false is on")
	}
}

func TestProxyCustomCertificates(t *testing.T) {
	cfg, err := proxyConfig(t, newWorld().withSecrets("CERT_PEM=certificate\nKEY_PEM=private_key"), "{ssl: {certificate_pem: CERT_PEM, private_key_pem: KEY_PEM}, host: example.com}")
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Proxy()
	if p.HostTLSCert() != ".kamal/proxy/apps-config/app/tls/cert.pem" || p.HostTLSKey() != ".kamal/proxy/apps-config/app/tls/key.pem" ||
		p.ContainerTLSCert() != "/home/kamal-proxy/.apps-config/app/tls/cert.pem" || p.ContainerTLSKey() != "/home/kamal-proxy/.apps-config/app/tls/key.pem" {
		t.Fatal("wrong TLS paths")
	}
	options := p.DeployOptions()
	if deployOption(options, "tls") != "true" || deployOption(options, "tls-certificate-path") != "/home/kamal-proxy/.apps-config/app/tls/cert.pem" ||
		deployOption(options, "tls-private-key-path") != "/home/kamal-proxy/.apps-config/app/tls/key.pem" {
		t.Fatal("wrong TLS deploy options")
	}
}

func TestProxyCertificateWithoutKey(t *testing.T) {
	for _, ssl := range []string{"{certificate_pem: CERT_PEM}", "{private_key_pem: KEY_PEM}"} {
		_, err := proxyConfig(t, newWorld(), "{ssl: "+ssl+", host: example.com}")
		assertConfigurationError(t, err)
	}
}

func TestProxyBootDirectories(t *testing.T) {
	b := baseConfig(t).ProxyBoot()
	got := []string{b.AppsDirectory(), b.AppsContainerDirectory(), b.AppDirectory(), b.AppContainerDirectory(),
		b.ErrorPagesDirectory(), b.ErrorPagesContainerDirectory(), b.TLSDirectory(), b.TLSContainerDirectory()}
	want := ".kamal/proxy/apps-config /home/kamal-proxy/.apps-config .kamal/proxy/apps-config/app /home/kamal-proxy/.apps-config/app " +
		".kamal/proxy/apps-config/app/error_pages /home/kamal-proxy/.apps-config/app/error_pages .kamal/proxy/apps-config/app/tls /home/kamal-proxy/.apps-config/app/tls"
	if joined(got) != want {
		t.Fatalf("directories = %s", joined(got))
	}
}

func TestProxyRunEquality(t *testing.T) {
	cfg := mustBuild(t, newWorld(), deepMerged(t, shortDeploy, "proxy: {run: {log_max_size: 50m}}"))
	a := &ProxyRun{cfg: cfg, config: parseYAML(t, "log_max_size: 50m")}
	b := &ProxyRun{cfg: cfg, config: parseYAML(t, "log_max_size: 50m")}
	c := &ProxyRun{cfg: cfg, config: parseYAML(t, "log_max_size: 100m")}
	if !a.Equal(b) || a.Equal(c) {
		t.Fatal("run equality is wrong")
	}
}

func TestNoConflictWhenGlobalProxyRunConfigIsInherited(t *testing.T) {
	mustBuild(t, newWorld(), deepMerged(t, shortDeploy, `
servers:
  web: {hosts: [1.1.1.1]}
  worker: {hosts: [1.1.1.1], cmd: bin/jobs, proxy: {hosts: [worker.example.com], app_port: 8080}}
proxy:
  hosts: [example.com]
  run: {log_max_size: "", options: {log-driver: journald}}
`))
}

func sshOf(t *testing.T, w *fakeWorld, ssh string) *SSH {
	t.Helper()
	return mustBuild(t, w, merged(t, baseDeploy, "ssh: "+ssh)).SSH()
}

func TestSSHOptions(t *testing.T) {
	if baseConfig(t).SSH().User() != "root" || sshOf(t, newWorld(), "{user: app}").User() != "app" {
		t.Fatal("wrong ssh user")
	}
	if sshOf(t, newWorld(), "{user: app}").LoggerLevel() != 4 || sshOf(t, newWorld(), "{log_level: debug}").LoggerLevel() != 0 {
		t.Fatal("wrong logger level")
	}
	if sshOf(t, newWorld(), "{port: 2222}").Port() != 2222 {
		t.Fatal("wrong ssh port")
	}
	configs := map[string]string{"true": "true", "false": "false", "'~/config.mine'": `"~/config.mine"`, "['~/config.mine.1', '~/config.mine.2']": `["~/config.mine.1", "~/config.mine.2"]`}
	for config, want := range configs {
		if got := sshOf(t, newWorld(), "{config: "+config+"}").Config().Inspect(); got != want {
			t.Errorf("config %s = %s", config, got)
		}
	}
	if !baseConfig(t).SSH().ForwardAgent().IsNull() || !sshOf(t, newWorld(), "{forward_agent: false}").ForwardAgent().IsFalse() || !sshOf(t, newWorld(), "{forward_agent: true}").ForwardAgent().IsTrue() {
		t.Fatal("wrong forward_agent")
	}
}

func TestSSHProxyHost(t *testing.T) {
	for proxy, jump := range map[string]string{"1.2.3.4": "root@1.2.3.4", "app@1.2.3.4": "app@1.2.3.4"} {
		got, ok := sshOf(t, newWorld(), "{proxy: '"+proxy+"'}").Proxy()
		if !ok || got.Jump != jump {
			t.Errorf("proxy %s = %+v", proxy, got)
		}
	}
}

func TestSSHKeyData(t *testing.T) {
	w := newWorld()
	var stderr bytes.Buffer
	env := w.environment()
	env.Stderr = &stderr
	cfg, err := New(merged(t, baseDeploy, "ssh: {key_data: ['-----BEGIN OPENSSH PRIVATE KEY-----']}"), Options{Env: env})
	if err != nil {
		t.Fatal(err)
	}
	keys, _, _ := cfg.SSH().KeyData()
	if joined(keys) != "-----BEGIN OPENSSH PRIVATE KEY-----" || !strings.Contains(stderr.String(), "Inline key_data usage is deprecated") {
		t.Fatalf("keys %v stderr %q", keys, stderr.String())
	}
	secretWorld := newWorld().withSecrets("SSH_PRIVATE_KEY=secret_ssh_key\nSECOND_KEY=second_secret_ssh_key")
	keys, _, _ = sshOf(t, secretWorld, "{key_data: [SSH_PRIVATE_KEY, SECOND_KEY]}").KeyData()
	if joined(keys) != "secret_ssh_key second_secret_ssh_key" {
		t.Fatalf("keys = %v", keys)
	}
}

func TestSshkit(t *testing.T) {
	defaults := mustBuild(t, newWorld(), parseYAML(t, baseDeploy)).Sshkit()
	retries, _ := defaults.DNSRetries()
	if defaults.MaxConcurrentStarts() != 30 || defaults.PoolIdleTimeout() != 900 || retries != 3 {
		t.Fatal("wrong sshkit defaults")
	}
	overrides := mustBuild(t, newWorld(), merged(t, baseDeploy, "sshkit: {max_concurrent_starts: 50, pool_idle_timeout: 600, dns_retries: 5}")).Sshkit()
	retries, _ = overrides.DNSRetries()
	if overrides.MaxConcurrentStarts() != 50 || overrides.PoolIdleTimeout() != 600 || retries != 5 {
		t.Fatal("sshkit overrides not applied")
	}
}

func TestVolumeDockerArgs(t *testing.T) {
	cases := map[Volume]string{
		{HostPath: "/root/foo/bar", ContainerPath: "/assets"}:                  "--volume /root/foo/bar:/assets",
		{HostPath: "foo/bar", ContainerPath: "/assets"}:                        "--volume $PWD/foo/bar:/assets",
		{HostPath: "/root/foo/bar", ContainerPath: "/assets", Options: "ro"}:   "--volume /root/foo/bar:/assets:ro",
		{HostPath: "/root/foo/bar", ContainerPath: "/assets", Options: "ro,z"}: "--volume /root/foo/bar:/assets:ro,z",
		{HostPath: "/data", ContainerPath: "/data", Options: "z"}:              "--volume /data:/data:z",
		{HostPath: "/data", ContainerPath: "/data", Options: "Z"}:              "--volume /data:/data:Z",
	}
	for volume, want := range cases {
		if got := volume.DockerArgs().String(); got != want {
			t.Errorf("%+v: %s", volume, got)
		}
	}
}
