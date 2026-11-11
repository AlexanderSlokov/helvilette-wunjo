package cli

// Ports test/cli/proxy_test.rb at Kamal 2.12.0.

import (
	"strings"
	"testing"
)

const bootConfigEcho = `echo $(cat .kamal/proxy/options 2> /dev/null || echo "--publish 80:80 --publish 443:443 --log-opt max-size=10m") $(cat .kamal/proxy/image 2> /dev/null || echo "basecamp/kamal-proxy"):$(cat .kamal/proxy/image_version 2> /dev/null || echo "v0.9.2") $(cat .kamal/proxy/run_command 2> /dev/null || echo "") | xargs docker run --name kamal-proxy --network kamal --detach --restart unless-stopped --volume kamal-proxy-config:/home/kamal-proxy/.config/kamal-proxy --volume $PWD/.kamal/proxy/apps-config:/home/kamal-proxy/.apps-config`

const proxyVersionCommand = `docker inspect kamal-proxy --format '{{.Config.Image}}' | awk -F: '{print $NF}'`

func proxyWorld(t *testing.T, fixture string) *world { return newWorld(t).fixture("deploy_" + fixture) }

func proxyRun(w *world, fixture string, args ...string) string {
	w.t.Helper()
	return w.run(append(append([]string{"proxy"}, args...), "-c", "test/fixtures/deploy_"+fixture+".yml")...)
}

func TestProxyBoot(t *testing.T) {
	output := proxyRun(proxyWorld(t, "with_proxy"), "with_proxy", "boot")
	assertContains(t, output, "docker login")
	assertContains(t, output, "mkdir -p .kamal/proxy/apps-config")
	assertContains(t, output, bootConfigEcho)
}

func TestProxyBootWithRunConfig(t *testing.T) {
	output := proxyRun(proxyWorld(t, "with_proxy_run_config"), "with_proxy_run_config", "boot")
	assertContains(t, output, "docker login")
	assertContains(t, output, "mkdir -p .kamal/proxy/apps-config")
	assertContains(t, output, "docker container start kamal-proxy || docker run --name kamal-proxy --network kamal --detach --restart unless-stopped --volume kamal-proxy-config:/home/kamal-proxy/.config/kamal-proxy --volume $PWD/.kamal/proxy/apps-config:/home/kamal-proxy/.apps-config --publish 80:80 --publish ")
}

func TestProxyBootWithRunConfigConflicts(t *testing.T) {
	w := proxyWorld(t, "with_proxy_run_config_conflicts")
	output := w.runFailing("proxy", "boot", "-c", "test/fixtures/deploy_with_proxy_run_config_conflicts.yml")
	assertContains(t, output, "ERROR (Kamal::ConfigurationError): Conflicting proxy run configurations for host 1.1.1.2")
}

func TestProxyBootOldVersion(t *testing.T) {
	w := proxyWorld(t, "with_proxy").stubCapture(equals(proxyVersionCommand), "v0.0.1")
	output := w.runFailing("proxy", "boot", "-c", "test/fixtures/deploy_with_proxy.yml")
	assertContains(t, output, "kamal-proxy version v0.0.1 is too old, run `kamal proxy reboot` in order to update to at least v0.9.2")
}

func TestProxyBootCorrectVersion(t *testing.T) {
	w := proxyWorld(t, "with_proxy").stubCapture(equals(proxyVersionCommand), "v0.9.2")
	output := proxyRun(w, "with_proxy", "boot")
	assertContains(t, output, "docker login")
	assertContains(t, output, "docker container start kamal-proxy || "+bootConfigEcho)
}

func TestProxyReboot(t *testing.T) {
	output := proxyRun(proxyWorld(t, "with_proxy"), "with_proxy", "reboot", "-y")
	for _, host := range []string{"1.1.1.1", "1.1.1.2"} {
		assertContains(t, output, "docker container stop kamal-proxy on "+host)
		assertContains(t, output, "docker container prune --force --filter label=org.opencontainers.image.title=kamal-proxy on "+host)
		assertContains(t, output, "mkdir -p .kamal/proxy/apps-config on "+host)
	}
	assertContains(t, output, bootConfigEcho)
}

func TestProxyRebootRolling(t *testing.T) {
	output := proxyRun(proxyWorld(t, "with_proxy"), "with_proxy", "reboot", "--rolling", "-y")
	assertContains(t, output, "Running docker container prune --force --filter label=org.opencontainers.image.title=kamal-proxy on 1.1.1.1")
}

func TestProxyStartStopRestartDetails(t *testing.T) {
	assertContains(t, proxyRun(proxyWorld(t, "with_proxy"), "with_proxy", "start"), "docker container start kamal-proxy")
	assertContains(t, proxyRun(proxyWorld(t, "with_proxy"), "with_proxy", "stop"), "docker container stop kamal-proxy")
	restart := proxyRun(proxyWorld(t, "with_proxy"), "with_proxy", "restart")
	assertContains(t, restart, "docker container stop kamal-proxy")
	assertContains(t, restart, "docker container start kamal-proxy")
	assertContains(t, proxyRun(proxyWorld(t, "with_proxy"), "with_proxy", "details"), "docker ps --filter 'name=^kamal-proxy$'")
}

func TestProxyLogs(t *testing.T) {
	w := proxyWorld(t, "with_proxy").stubCapture(equals("docker logs kamal-proxy --tail 100 --timestamps 2>&1"), "Log entry")
	output := proxyRun(w, "with_proxy", "logs")
	assertContains(t, output, "Proxy Host: 1.1.1.1")
	assertContains(t, output, "Log entry")
}

func TestProxyLogsWithFollow(t *testing.T) {
	w := proxyWorld(t, "with_proxy")
	assertContains(t, proxyRun(w, "with_proxy", "logs", "--follow"), "docker logs kamal-proxy --timestamps --tail 10 --follow")
	if len(w.execs) != 1 || w.execs[0] != "ssh -t root@1.1.1.1 -p 22 'docker logs kamal-proxy --timestamps --tail 10 --follow 2>&1'" {
		t.Fatalf("exec = %v", w.execs)
	}
}

func TestProxyRemove(t *testing.T) {
	output := proxyRun(proxyWorld(t, "with_proxy"), "with_proxy", "remove")
	assertContains(t, output, "/usr/bin/env ls .kamal/apps | wc -l")
	assertContains(t, output, "docker container prune --force --filter label=org.opencontainers.image.title=kamal-proxy")
	assertContains(t, output, "docker image prune --all --force --filter label=org.opencontainers.image.title=kamal-proxy")
}

func TestProxyRemoveWithOtherApps(t *testing.T) {
	w := proxyWorld(t, "with_proxy").stubCapture(equals("/usr/bin/env ls .kamal/apps | wc -l"), "1\n")
	output := proxyRun(w, "with_proxy", "remove")
	assertContains(t, output, "Not removing the proxy, as other apps are installed, ignore this check with kamal proxy remove --force")
	assertNotContains(t, output, "docker image prune --all --force")
}

func TestForceRemoveWithOtherApps(t *testing.T) {
	w := proxyWorld(t, "with_proxy").stubCapture(equals("/usr/bin/env ls .kamal/apps | wc -l"), "1\n")
	output := proxyRun(w, "with_proxy", "remove", "--force")
	assertContains(t, output, "Forcing, so removing the proxy, even though other apps are installed")
	assertContains(t, output, "docker image prune --all --force --filter label=org.opencontainers.image.title=kamal-proxy")
}

func TestProxyRemoveContainerAndImage(t *testing.T) {
	assertContains(t, proxyRun(proxyWorld(t, "with_proxy"), "with_proxy", "remove_container"), "docker container prune --force --filter label=org.opencontainers.image.title=kamal-proxy")
	assertContains(t, proxyRun(proxyWorld(t, "with_proxy"), "with_proxy", "remove_image"), "docker image prune --all --force --filter label=org.opencontainers.image.title=kamal-proxy")
}

func upgradeWorld(t *testing.T) *world {
	return proxyWorld(t, "with_proxy").
		stubCapture(equals(proxyVersionCommand), "v0.9.2").
		stubCapture(equals(`docker container ls --all --filter 'name=^app-workers-latest$' --quiet | xargs docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}'`), "running").
		stubCapture(anything, "12345678")
}

func TestProxyUpgrade(t *testing.T) {
	output := proxyRun(upgradeWorld(t), "with_proxy", "upgrade", "-y")
	for _, want := range []string{
		"Upgrading proxy on 1.1.1.1,1.1.1.2,1.1.1.3,1.1.1.4...", "docker login -u [REDACTED] -p [REDACTED]",
		"docker container stop traefik ; docker container prune --force --filter label=org.opencontainers.image.title=Traefik && docker image prune --all --force --filter label=org.opencontainers.image.title=Traefik",
		"docker container stop kamal-proxy", "docker container prune --force --filter label=org.opencontainers.image.title=kamal-proxy",
		"docker image prune --all --force --filter label=org.opencontainers.image.title=kamal-proxy", "/usr/bin/env mkdir -p .kamal",
		"docker network create kamal", "docker container start kamal-proxy || " + bootConfigEcho,
		"/usr/bin/env mkdir -p .kamal/apps/app/env/roles", `Uploading "\n" to .kamal/apps/app/env/roles/web.env`,
		`docker exec kamal-proxy kamal-proxy deploy app-web --target="12345678:80" --deploy-timeout="6s" --drain-timeout="30s" --buffer-requests --buffer-responses --log-request-header="Cache-Control" --log-request-header="Last-Modified" --log-request-header="User-Agent"`,
		"docker container ls --all --filter 'name=^app-web-12345678$' --quiet | xargs docker stop", "docker tag dhh/app:latest dhh/app:latest",
		"docker ps -q -a --filter label=service=app --filter label=destination= --filter status=created --filter status=exited --filter status=dead | tail -n +6 | while read container_id; do docker rm $container_id; done",
		"docker image prune --force --filter label=service=app", "Upgraded proxy on 1.1.1.1,1.1.1.2,1.1.1.3,1.1.1.4",
	} {
		assertContains(t, output, want)
	}
	assertMatches(t, output, `docker rename app-web-latest app-web-latest_replaced_.*`)
	assertMatches(t, output, `docker run --detach --restart unless-stopped --name app-web-latest --network kamal --hostname 1\.1\.1\.1-.* --env KAMAL_CONTAINER_NAME="app-web-latest" --env KAMAL_VERSION="latest" --env KAMAL_HOST="1\.1\.1\.1" --env-file \.kamal/apps/app/env/roles/web\.env --log-opt max-size="10m"`)
}

func TestProxyUpgradeRolling(t *testing.T) {
	output := proxyRun(upgradeWorld(t), "with_proxy", "upgrade", "--rolling", "-y")
	for _, host := range allHosts {
		assertContains(t, output, "Upgrading proxy on "+host+"...")
		assertContains(t, output, "docker container stop traefik ; docker container prune --force --filter label=org.opencontainers.image.title=Traefik && docker image prune --all --force --filter label=org.opencontainers.image.title=Traefik on "+host)
		assertContains(t, output, "Upgraded proxy on "+host)
	}
}

// bootConfigCase is one `proxy boot_config set` run and its expected writes per
// host; an empty upload means the file is removed.
type bootConfigCase struct {
	name                                     string
	args                                     []string
	options, image, imageVersion, runCommand string
}

func TestProxyBootConfigSet(t *testing.T) {
	defaults := "--publish 80:80 --publish 443:443 --log-opt max-size=10m"
	cases := []bootConfigCase{
		{name: "boot_config set"},
		{name: "boot_config set no publish", args: []string{"--publish", "false"}, options: "--log-opt max-size=10m"},
		{name: "boot_config set custom max_size", args: []string{"--log-max-size", "100m"}, options: "--publish 80:80 --publish 443:443 --log-opt max-size=100m"},
		{name: "boot_config set no log max size", args: []string{"--log-max-size="}, options: "--publish 80:80 --publish 443:443"},
		{name: "boot_config set custom ports", args: []string{"--http-port", "8080", "--https-port", "8443"}, options: "--publish 8080:80 --publish 8443:443 --log-opt max-size=10m"},
		{name: "boot_config set bind IP", args: []string{"--publish-host-ip", "127.0.0.1"}, options: "--publish 127.0.0.1:80:80 --publish 127.0.0.1:443:443 --log-opt max-size=10m"},
		{name: "boot_config set multiple bind IPs", args: []string{"--publish-host-ip", "127.0.0.1", "--publish-host-ip", "::1"}, options: "--publish 127.0.0.1:80:80 --publish 127.0.0.1:443:443 --publish [::1]:80:80 --publish [::1]:443:443 --log-opt max-size=10m"},
		{name: "boot_config set docker options", args: []string{"--docker_options", "label=foo=bar", "add_host=thishost:thathost"}, options: defaults + " --label=foo=bar --add_host=thishost:thathost"},
		{name: "boot_config set registry", args: []string{"--registry", "myreg"}, image: "myreg/basecamp/kamal-proxy"},
		{name: "boot_config set repository", args: []string{"--repository", "myrepo"}, image: "myrepo/kamal-proxy"},
		{name: "boot_config set image_version", args: []string{"--image_version", "0.9.9"}, imageVersion: "0.9.9"},
		{name: "boot_config set run_command", args: []string{"--metrics_port", "9000", "--debug", "true"}, options: defaults + " --expose=9000", runCommand: `kamal-proxy run --debug --metrics-port \"9000\"`},
		{name: "boot_config set all", args: []string{"--docker_options", "label=foo=bar", "--registry", "myreg", "--repository", "myrepo", "--image_version", "0.9.9", "--metrics_port", "9000", "--debug", "true"},
			options: defaults + " --expose=9000 --label=foo=bar", image: "myreg/myrepo/kamal-proxy", imageVersion: "0.9.9", runCommand: `kamal-proxy run --debug --metrics-port \"9000\"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { checkBootConfig(t, c) })
	}
}

func checkBootConfig(t *testing.T, c bootConfigCase) {
	output := proxyRun(proxyWorld(t, "with_proxy"), "with_proxy", append([]string{"boot_config", "set"}, c.args...)...)
	files := []struct{ path, upload string }{
		{".kamal/proxy/options", c.options}, {".kamal/proxy/image", c.image},
		{".kamal/proxy/image_version", c.imageVersion}, {".kamal/proxy/run_command", c.runCommand},
	}
	for _, host := range []string{"1.1.1.1", "1.1.1.2"} {
		assertContains(t, output, "Running /usr/bin/env mkdir -p .kamal/proxy on "+host)
		for _, file := range files {
			if file.upload == "" {
				assertContains(t, output, "Running /usr/bin/env rm "+file.path+" on "+host)
			} else {
				assertContains(t, output, `Uploading "`+file.upload+`" to `+file.path+" on "+host)
			}
		}
	}
}

func TestProxyBootConfigSetInvalidBindIPs(t *testing.T) {
	w := proxyWorld(t, "with_proxy")
	output := w.runFailing("proxy", "boot_config", "set", "--publish-host-ip", "1.2.3.invalidIP", "--publish-host-ip", "::1", "-c", "test/fixtures/deploy_with_proxy.yml")
	assertContains(t, output, "Invalid publish IP address: 1.2.3.invalidIP")
}

func TestProxyBootConfigGet(t *testing.T) {
	echo := strings.TrimSuffix(bootConfigEcho, " | xargs docker run --name kamal-proxy --network kamal --detach --restart unless-stopped --volume kamal-proxy-config:/home/kamal-proxy/.config/kamal-proxy --volume $PWD/.kamal/proxy/apps-config:/home/kamal-proxy/.apps-config")
	w := proxyWorld(t, "with_proxy").stubCapture(equals("/usr/bin/env "+echo), "--publish 80:80 --publish 8443:443 --label=foo=bar basecamp/kamal-proxy:v1.0.0")
	output := proxyRun(w, "with_proxy", "boot_config", "get")
	for _, host := range []string{"1.1.1.1", "1.1.1.2"} {
		assertContains(t, output, "Host "+host+": --publish 80:80 --publish 8443:443 --label=foo=bar basecamp/kamal-proxy:v1.0.0")
	}
}

func TestProxyBootConfigReset(t *testing.T) {
	output := proxyRun(proxyWorld(t, "with_proxy"), "with_proxy", "boot_config", "reset")
	for _, host := range []string{"1.1.1.1", "1.1.1.2"} {
		assertContains(t, output, "rm .kamal/proxy/options on "+host)
	}
}
