package cli

// Ports test/cli/app_test.rb at Kamal 2.12.0. Where Ruby counts calls to `on`, the
// Go tests check which commands ran on which hosts.

import (
	"strings"
	"testing"
	"time"
)

const (
	currentVersionCommand = `/usr/bin/env sh -c 'docker ps --latest --format '\''{{.Names}}'\'' --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''{{.ID}}'\'') ; docker ps --latest --format '\''{{.Names}}'\'' --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting' | head -1 | while read line; do echo ${line#app-web-}; done`
	webLatestID           = "docker container ls --all --filter 'name=^app-web-latest$' --quiet"
	workersHealth         = `docker container ls --all --filter 'name=^app-workers-latest$' --quiet | xargs docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}'`
	webRunPrefix          = `docker run --detach --restart unless-stopped --name app-web-latest --network kamal --hostname 1\.1\.1\.1-[0-9a-f]{12} `
	stopOld               = "docker container ls --all --filter 'name=^app-web-123$' --quiet | xargs docker stop"
	runningContainer      = `sh -c 'docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''{{.ID}}'\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting' | head -1`
)

func appWorld(t *testing.T, config string) *world { return newWorld(t).fixture("deploy_" + config) }

// appRun runs `app <args> -c deploy_<config>.yml [--hosts host]`, as run_command.
func appRun(w *world, config, host string, args ...string) string {
	w.t.Helper()
	return w.run(appArgs(config, host, args)...)
}

func appArgs(config, host string, args []string) []string {
	full := append(append([]string{"app"}, args...), "-c", "test/fixtures/deploy_"+config+".yml")
	if host != "" {
		full = append(full, "--hosts", host)
	}
	return full
}

// stubRunning returns "123" for every capture, as stub_running does. Health checks
// report running, because the Ruby tests that count `on` calls never reach them.
func stubRunning(w *world) *world {
	return w.stubCapture(contains("| xargs docker inspect --format"), "running").stubCapture(anything, "123")
}

func TestAppBoot(t *testing.T) {
	output := appRun(stubRunning(appWorld(t, "with_accessories")), "with_accessories", "1.1.1.1", "boot")
	assertContains(t, output, "docker tag dhh/app:latest dhh/app:latest")
	assertMatches(t, output, webRunPrefix)
	assertContains(t, output, stopOld)
}

func TestAppBootThroughMainNamesTheSubcommandToHooks(t *testing.T) {
	w := stubRunning(appWorld(t, "with_accessories")).hooksExist()
	w.run("app", "boot", "-c", "test/fixtures/deploy_with_accessories.yml", "--hosts", "1.1.1.1")
	env := w.local.processEnvOf(".kamal/hooks/pre-app-boot")
	if env["KAMAL_COMMAND"] != "app" || env["KAMAL_SUBCOMMAND"] != "boot" {
		t.Fatalf("pre-app-boot env = %v", env)
	}
}

func TestAppBootWillRenameIfSameVersionIsAlreadyRunning(t *testing.T) {
	w := appWorld(t, "with_accessories").stubCapture(equals(webLatestID), "12345678").stubCapture(equals(currentVersionCommand), "123")
	output := appRun(w, "with_accessories", "1.1.1.1", "boot")
	assertMatches(t, output, `Renaming container .* to .* as already deployed on 1\.1\.1\.1`)
	assertMatches(t, output, `docker rename app-web-latest app-web-latest_replaced_[0-9a-f]{16}`)
	assertMatches(t, output, webRunPrefix)
	assertContains(t, output, stopOld)
}

func TestAppBootUsesGroupStrategyWhenSpecified(t *testing.T) {
	w := stubRunning(appWorld(t, "with_boot_strategy")).hooksExist()
	output := appRun(w, "with_boot_strategy", "", "boot")
	assertHookRan(t, output, "pre-app-boot", 2)
	assertHookRan(t, output, "post-app-boot", 2)
	waits := 0
	for _, d := range w.sleeps {
		if d == 2*time.Second {
			waits++
		}
	}
	if waits != 2 {
		t.Fatalf("sleeps = %v, want two of 2s", w.sleeps)
	}
	groups := w.local.processEnvValues(".kamal/hooks/pre-app-boot", "KAMAL_HOSTS")
	if strings.Join(groups, " | ") != "1.1.1.1,1.1.1.2,1.1.1.3 | 1.1.1.4" {
		t.Fatalf("boot groups = %v", groups)
	}
}

func TestAppBootWithAndWithoutParallelRoles(t *testing.T) {
	for _, config := range []string{"without_parallel_roles", "with_parallel_roles"} {
		w := stubRunning(appWorld(t, config))
		output := appRun(w, config, "", "boot")
		for _, run := range []string{"app-web-latest --network kamal --hostname 1.1.1.1", "app-web-latest --network kamal --hostname 1.1.1.2",
			"app-workers-latest --network kamal --hostname 1.1.1.1", "app-workers-latest --network kamal --hostname 1.1.1.3"} {
			assertContains(t, output, run)
		}
	}
}

func TestAppBootErrorsDontLeaveLockInPlace(t *testing.T) {
	w := appWorld(t, "with_accessories").stubCapture(anything, "").stubRemote(contains("docker run --detach"), "", "boom")
	output := w.runFailing(appArgs("with_accessories", "1.1.1.1", []string{"boot"})...)
	assertContains(t, output, "Releasing the deploy lock...")
}

func TestAppBootWithAssets(t *testing.T) {
	w := appWorld(t, "with_assets").stubCapture(equals(webLatestID), "12345678").stubCapture(equals(currentVersionCommand), "123")
	output := appRun(w, "with_assets", "1.1.1.1", "boot")
	assertContains(t, output, "docker tag dhh/app:latest dhh/app:latest")
	assertContains(t, output, "/usr/bin/env mkdir -p .kamal/apps/app/assets/volumes/web-latest ; cp -rnT .kamal/apps/app/assets/extracted/web-latest .kamal/apps/app/assets/volumes/web-latest ; cp -rnT .kamal/apps/app/assets/extracted/web-latest .kamal/apps/app/assets/volumes/web-123 || true ; cp -rnT .kamal/apps/app/assets/extracted/web-123 .kamal/apps/app/assets/volumes/web-latest || true")
	assertContains(t, output, "/usr/bin/env mkdir -p .kamal/apps/app/assets/extracted/web-latest && docker container rm app-web-assets 2> /dev/null || true && docker container create --name app-web-assets dhh/app:latest && docker container cp -L app-web-assets:/public/assets/. .kamal/apps/app/assets/extracted/web-latest && docker container rm app-web-assets")
	assertMatches(t, output, webRunPrefix)
	assertContains(t, output, stopOld)
	assertContains(t, output, `/usr/bin/env find .kamal/apps/app/assets/extracted -maxdepth 1 -name 'web-*' ! -name web-latest -exec rm -rf "{}" + ; find .kamal/apps/app/assets/volumes -maxdepth 1 -name 'web-*' ! -name web-latest -exec rm -rf "{}" +`)
}

func TestAppBootWithHostTags(t *testing.T) {
	w := appWorld(t, "with_env_tags").stubCapture(equals(webLatestID), "12345678").stubCapture(equals(currentVersionCommand), "123")
	output := appRun(w, "with_env_tags", "1.1.1.1", "boot")
	assertMatches(t, output, `docker run --detach --restart unless-stopped --name app-web-latest --network kamal --hostname 1\.1\.1\.1-[0-9a-f]{12} --env KAMAL_CONTAINER_NAME="app-web-latest" --env KAMAL_VERSION="latest" --env KAMAL_HOST="1\.1\.1\.1" --env TEST="root" --env EXPERIMENT="disabled" --env SITE="site1"`)
	assertContains(t, output, stopOld)
}

func rolesBootWorld(t *testing.T, workers string) *world {
	return appWorld(t, "with_roles").stubCapture(equals(workersHealth), workers).stubCapture(anything, "123")
}

func TestAppBootWithWebBarrierOpened(t *testing.T) {
	output := appRun(rolesBootWorld(t, "running"), "with_roles", "", "boot")
	for _, host := range []string{"1.1.1.3", "1.1.1.4"} {
		assertContains(t, output, "Waiting for the first healthy web container before booting workers on "+host+"...")
		assertContains(t, output, "First web container is healthy, booting workers on "+host)
	}
}

func TestAppBootWithWebBarrierClosed(t *testing.T) {
	w := appWorld(t, "with_roles").stubCapture(anything, "123").
		stubRemote(contains("docker exec kamal-proxy kamal-proxy deploy app-web"), "", "boom")
	output := w.runFailing(appArgs("with_roles", "", []string{"boot"})...)
	for _, host := range []string{"1.1.1.3", "1.1.1.4"} {
		assertContains(t, output, "Waiting for the first healthy web container before booting workers on "+host+"...")
		assertContains(t, output, "First web container is unhealthy, not booting workers on "+host)
	}
}

func TestAppBootWithWorkerErrors(t *testing.T) {
	output := rolesBootWorld(t, "unhealthy").runFailing(appArgs("with_roles", "", []string{"boot"})...)
	for _, host := range []string{"1.1.1.3", "1.1.1.4"} {
		assertContains(t, output, "First web container is healthy, booting workers on "+host)
		assertContains(t, output, "ERROR Failed to boot workers on "+host)
	}
}

func TestAppBootWithWorkerReadyThenNot(t *testing.T) {
	w := appWorld(t, "with_roles")
	w.hosts.stubs = append(w.hosts.stubs, sequenceStub(equals(workersHealth), "running", "stopped"))
	w.stubCapture(anything, "123")
	assertContains(t, w.runFailing(appArgs("with_roles", "1.1.1.3", []string{"boot"})...), "ERROR Failed to boot workers on 1.1.1.3")
}

func TestAppBootWithOnlyWorkers(t *testing.T) {
	output := appRun(appWorld(t, "with_only_workers").stubCapture(equals(workersHealth), "running").stubCapture(anything, "123"), "with_only_workers", "", "boot")
	assertMatches(t, output, `First workers container is healthy on 1\.1\.1\.\d, booting any other roles`)
	assertNotContains(t, output, "kamal-proxy")
}

func TestAppBootWithErrorPages(t *testing.T) {
	w := stubRunning(appWorld(t, "with_error_pages"))
	w.files["/k/public/404.html"], w.files["/k/public/503.html"] = "404 page", "503 page"
	output := appRun(w, "with_error_pages", "1.1.1.1", "boot")
	assertMatches(t, output, `Uploading .*kamal-error-pages.*/latest to \.kamal/proxy/apps-config/app/error_pages`)
	assertContains(t, output, "docker tag dhh/app:latest dhh/app:latest")
	assertMatches(t, output, webRunPrefix)
	assertContains(t, output, stopOld)
	assertContains(t, output, "Running /usr/bin/env find .kamal/proxy/apps-config/app/error_pages -mindepth 1 -maxdepth 1 ! -name latest -exec rm -rf {} + on 1.1.1.1")
}

func TestAppBootWithCustomSSLCertificate(t *testing.T) {
	w := stubRunning(appWorld(t, "with_proxy"))
	w.files["/k/test/fixtures/deploy_with_proxy.yml"] += "\nproxy:\n  host: example.com\n  ssl:\n    certificate_pem: CERT_PEM\n    private_key_pem: KEY_PEM\n"
	w.files[".kamal/secrets"] = "CERT_PEM=CERTIFICATE CONTENT\nKEY_PEM=PRIVATE KEY CONTENT\n"
	output := appRun(w, "with_proxy", "1.1.1.1", "boot")
	assertContains(t, output, "Writing SSL certificates for web on 1.1.1.1")
	assertContains(t, output, "mkdir -p .kamal/proxy/apps-config/app/tls")
	assertContains(t, output, `Uploading "CERTIFICATE CONTENT" to .kamal/proxy/apps-config/app/tls/web/cert.pem`)
	assertContains(t, output, `--tls-certificate-path="/home/kamal-proxy/.apps-config/app/tls/web/cert.pem"`)
	assertContains(t, output, `--tls-private-key-path="/home/kamal-proxy/.apps-config/app/tls/web/key.pem"`)
}

func TestAppStartAndStop(t *testing.T) {
	output := appRun(appWorld(t, "with_accessories").stubCapture(anything, "999"), "with_accessories", "1.1.1.1", "start")
	assertContains(t, output, "docker start app-web-999")
	assertContains(t, output, `docker exec kamal-proxy kamal-proxy deploy app-web --target="999:80" --deploy-timeout="30s" --drain-timeout="30s" --buffer-requests --buffer-responses`)
	assertContains(t, appRun(appWorld(t, "with_accessories"), "with_accessories", "1.1.1.1", "stop"), runningContainer+" | xargs docker stop")
}

func staleWorld(t *testing.T) *world {
	return appWorld(t, "with_accessories").
		stubCapture(contains(`docker ps --filter label=service=app --filter label=destination= --filter label=role=web --format "{{.Names}}"`), "12345678\n87654321\n").
		stubCapture(equals(currentVersionCommand), "12345678\n")
}

func TestAppStaleContainers(t *testing.T) {
	assertMatches(t, appRun(staleWorld(t), "with_accessories", "1.1.1.1", "stale_containers"), `Detected stale container for role web with version 87654321`)
	stopped := appRun(staleWorld(t), "with_accessories", "1.1.1.1", "stale_containers", "--stop")
	assertContains(t, stopped, "Stopping stale container for role web with version 87654321")
	assertContains(t, stopped, "docker container ls --all --filter 'name=^app-web-87654321$' --quiet | xargs docker stop")
}

func TestAppDetailsContainersImages(t *testing.T) {
	assertContains(t, appRun(appWorld(t, "with_accessories"), "with_accessories", "1.1.1.1", "details"), "docker ps --filter label=service=app --filter label=destination= --filter label=role=web")
	assertContains(t, appRun(appWorld(t, "with_accessories"), "with_accessories", "1.1.1.1", "containers"), "docker container ls --all --filter label=service=app")
	assertContains(t, appRun(appWorld(t, "with_accessories"), "with_accessories", "1.1.1.1", "images"), "docker image ls dhh/app")
}

func TestAppRemove(t *testing.T) {
	output := appRun(appWorld(t, "with_accessories").hooksExist(), "with_accessories", "1.1.1.1", "remove")
	assertHookRan(t, output, "pre-app-remove", 1)
	for _, want := range []string{runningContainer + " | xargs docker stop", "docker container prune --force --filter label=service=app",
		"docker image prune --all --force --filter label=service=app", "rm -r .kamal/apps/app on 1.1.1.1", "rm -r .kamal/proxy/apps-config/app on 1.1.1.1"} {
		assertContains(t, output, want)
	}
	assertHookRan(t, output, "post-app-remove", 1)
}

func TestAppRemoveWithRoleFilter(t *testing.T) {
	partial := appRun(appWorld(t, "with_two_roles_one_host"), "with_two_roles_one_host", "1.1.1.1", "remove", "-r", "workers")
	assertContains(t, partial, "docker stop")
	assertContains(t, partial, "docker container prune --force --filter label=service=app")
	assertNotContains(t, partial, "docker image prune --all --force --filter label=service=app")
	assertNotContains(t, partial, "rm -r .kamal/apps/app")
	full := appRun(appWorld(t, "with_two_roles_one_host"), "with_two_roles_one_host", "1.1.1.1", "remove", "-r", "workers,web")
	assertContains(t, full, "docker image prune --all --force --filter label=service=app")
	assertContains(t, full, "rm -r .kamal/apps/app on 1.1.1.1")
	assertContains(t, full, "rm -r .kamal/proxy/apps-config/app on 1.1.1.1")
}

func TestAppRemoveHiddenCommands(t *testing.T) {
	cases := map[string][]string{
		"docker container ls --all --filter 'name=^app-web-1234567$' --quiet | xargs docker container rm": {"remove_container", "1234567"},
		"docker container prune --force --filter label=service=app":                                       {"remove_containers"},
		"docker image prune --all --force --filter label=service=app":                                     {"remove_images"},
		"rm -r .kamal/apps/app on 1.1.1.1":                                                                {"remove_app_directories"},
	}
	for want, args := range cases {
		assertContains(t, appRun(appWorld(t, "with_accessories"), "with_accessories", "1.1.1.1", args...), want)
	}
}

const execRunPattern = `docker run --rm --name app-web-exec-latest-[0-9a-f]{6} --network kamal --env-file \.kamal/apps/app/env/roles/web\.env --log-opt max-size="10m" dhh/app:latest ruby -v`

func TestAppExec(t *testing.T) {
	output := appRun(appWorld(t, "with_accessories"), "with_accessories", "1.1.1.1", "exec", "ruby -v")
	assertContains(t, output, "docker login -u [REDACTED] -p [REDACTED]")
	assertMatches(t, output, execRunPattern)
	assertMatches(t, appRun(appWorld(t, "with_accessories"), "with_accessories", "1.1.1.1", "exec", "ruby", " -v"), execRunPattern)
	assertMatches(t, appRun(appWorld(t, "with_accessories"), "with_accessories", "1.1.1.1", "exec", "--detach", "ruby -v"), strings.Replace(execRunPattern, "--rm", "--detach", 1))
}

func TestAppExecArgumentErrors(t *testing.T) {
	cases := map[string][]string{
		"No command provided. You must specify a command to execute.": {"exec"},
		"Detach is not compatible with reuse":                         {"exec", "--detach", "--reuse", "ruby -v"},
		"Detach is not compatible with interactive":                   {"exec", "--interactive", "--detach", "ruby -v"},
		"Detach is not compatible with interactive or reuse":          {"exec", "--interactive", "--detach", "--reuse", "ruby -v"},
	}
	for want, args := range cases {
		assertContains(t, appWorld(t, "with_accessories").runFailing(appArgs("with_accessories", "1.1.1.1", args)...), "ERROR (ArgumentError): "+want)
	}
}

func TestAppExecWithReuse(t *testing.T) {
	output := appRun(appWorld(t, "with_accessories").stubCapture(equals(currentVersionCommand), "999"), "with_accessories", "1.1.1.1", "exec", "--reuse", "ruby -v")
	assertContains(t, output, "docker exec app-web-999 ruby -v")
}

func TestAppExecInteractive(t *testing.T) {
	w := appWorld(t, "with_accessories").hooksExist()
	w.tty = true
	output := appRun(w, "with_accessories", "1.1.1.1", "exec", "-i", "ruby -v")
	assertHookRan(t, output, "pre-connect", 1)
	assertContains(t, output, "docker login -u [REDACTED] -p [REDACTED]")
	assertContains(t, output, "Get most recent version available as an image...")
	assertContains(t, output, "Launching interactive command with version latest via SSH from new container on 1.1.1.1...")
	assertMatches(t, strings.Join(w.execs, "\n"), `ssh -t root@1\.1\.1\.1 -p 22 'docker run -it --rm --name app-web-exec-latest-[0-9a-f]{6} --network kamal --env-file \.kamal/apps/app/env/roles/web\.env --log-opt max-size="10m" dhh/app:latest ruby -v'`)
}

func TestAppExecInteractiveWithReuse(t *testing.T) {
	for tty, flag := range map[bool]string{true: "-it", false: "-i"} {
		w := appWorld(t, "with_accessories").hooksExist().stubCapture(equals(currentVersionCommand), "999")
		w.tty = tty
		output := appRun(w, "with_accessories", "1.1.1.1", "exec", "-i", "--reuse", "ruby -v")
		assertHookRan(t, output, "pre-connect", 1)
		assertContains(t, output, "Launching interactive command with version 999 via SSH from existing container on 1.1.1.1...")
		if len(w.execs) != 1 || w.execs[0] != "ssh -t root@1.1.1.1 -p 22 'docker exec "+flag+" app-web-999 ruby -v'" {
			t.Fatalf("tty %v: exec = %v", tty, w.execs)
		}
	}
}

func TestAppLogs(t *testing.T) {
	output := appRun(appWorld(t, "with_accessories"), "with_accessories", "1.1.1.1", "logs")
	assertContains(t, output, runningContainer+" | xargs docker logs --timestamps --tail 100 2>&1")
	follows := map[string][]string{
		"--tail 10 --follow 2>&1":                   {"--follow"},
		"--timestamps --follow 2>&1 | grep \"hey\"": {"--follow", "--grep", "hey"},
		"--follow 2>&1 | grep \"hey\" -C 2":         {"--follow", "--grep", "hey", "--grep-options", "-C 2"},
		"--timestamps --since 5m --follow 2>&1":     {"--follow", "--since", "5m"},
	}
	for suffix, args := range follows {
		w := appWorld(t, "with_accessories")
		appRun(w, "with_accessories", "1.1.1.1", append([]string{"logs"}, args...)...)
		if len(w.execs) != 1 || !strings.HasSuffix(w.execs[0], suffix+"'") {
			t.Errorf("%v: exec = %v", args, w.execs)
		}
	}
	w := appWorld(t, "with_accessories")
	appRun(w, "with_accessories", "1.1.1.1", "logs", "--follow", "--container-id", "ID123")
	if len(w.execs) != 1 || w.execs[0] != `ssh -t root@1.1.1.1 -p 22 'sh -c '\''echo ID123'\'' | xargs docker logs --timestamps --tail 10 --follow 2>&1'` {
		t.Fatalf("exec = %v", w.execs)
	}
}

func TestAppVersion(t *testing.T) {
	versionCommand := strings.TrimPrefix(currentVersionCommand, "/usr/bin/env ")
	assertContains(t, appRun(appWorld(t, "with_accessories"), "with_accessories", "1.1.1.1", "version"), versionCommand)
}

func TestAppLongHostnames(t *testing.T) {
	cases := map[string]string{
		"this-hostname-is-really-unacceptably-long-to-be-honest.example.com": `--hostname this-hostname-is-really-unacceptably-long-to-be-hon-[0-9a-f]{12} `,
		"this-hostname-with-random-part-is-too-long.example.com":             `--hostname this-hostname-with-random-part-is-too-long\.example-[0-9a-f]{12} `,
	}
	for host, want := range cases {
		assertMatches(t, appRun(stubRunning(appWorld(t, "with_uncommon_hostnames")), "with_uncommon_hostnames", host, "boot"), want)
	}
}

func TestAppBootProxy(t *testing.T) {
	output := appRun(appWorld(t, "with_proxy").stubCapture(anything, "123"), "with_proxy", "1.1.1.1", "boot")
	assertMatches(t, output, `Renaming container .* to .* as already deployed on 1\.1\.1\.1`)
	assertMatches(t, output, `docker exec kamal-proxy kamal-proxy deploy app-web --target="123:80"`)
	assertContains(t, output, stopOld)
	roles := appRun(appWorld(t, "with_proxy_roles").stubCapture(anything, "123"), "with_proxy_roles", "", "boot")
	assertContains(t, roles, `docker exec kamal-proxy kamal-proxy deploy app-web --target="123:80" --deploy-timeout="6s" --drain-timeout="30s" --target-timeout="10s"`)
	assertContains(t, roles, `docker exec kamal-proxy kamal-proxy deploy app-web2 --target="123:80" --deploy-timeout="6s" --drain-timeout="30s" --target-timeout="15s"`)
}

func TestAppLiveAndMaintenance(t *testing.T) {
	assertContains(t, appRun(appWorld(t, "with_accessories"), "with_accessories", "1.1.1.1", "live"), "docker exec kamal-proxy kamal-proxy resume app-web on 1.1.1.1")
	assertContains(t, appRun(appWorld(t, "with_accessories"), "with_accessories", "1.1.1.1", "maintenance"), `docker exec kamal-proxy kamal-proxy stop app-web --drain-timeout="30s" on 1.1.1.1`)
	assertContains(t, appRun(appWorld(t, "with_accessories"), "with_accessories", "1.1.1.1", "maintenance", "--message", "Hello", "--drain_timeout", "10"), `docker exec kamal-proxy kamal-proxy stop app-web --drain-timeout="10s" --message="Hello" on 1.1.1.1`)
}
