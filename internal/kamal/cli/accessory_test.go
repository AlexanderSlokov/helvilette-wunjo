package cli

// Ports test/cli/accessory_test.rb at Kamal 2.12.0. Where Ruby expects calls to the
// directories, upload, stop, or boot methods, the Go tests check their commands in
// the output.

import (
	"strings"
	"testing"
)

const registriesConfig = "test/fixtures/deploy_with_accessories_with_different_registries.yml"

func accessoryWorld(t *testing.T) *world {
	w := newWorld(t).fixture("deploy_with_accessories_with_different_registries")
	w.files["/k/test/fixtures/files/my.cnf"] = "[mysqld]\n"
	w.files[".kamal/secrets"] = "MYSQL_ROOT_PASSWORD=secret"
	return w
}

func accessoryRun(w *world, args ...string) string {
	w.t.Helper()
	return w.run(append(append([]string{"accessory"}, args...), "-c", registriesConfig)...)
}

const mysqlRun = `docker run --name app-mysql --detach --restart unless-stopped --network kamal --log-opt max-size="10m" --publish 127.0.0.1:3306:3306 --env KAMAL_HOST="1.1.1.3" --env MYSQL_ROOT_HOST="%" --env-file .kamal/apps/app/env/accessories/mysql.env --volume $PWD/app-mysql/etc/mysql/my.cnf:/etc/mysql/my.cnf --volume $PWD/app-mysql/data:/var/lib/mysql --label service="app-mysql" private.registry/mysql:5.7 on 1.1.1.3`

func redisRun(host string) string {
	return `docker run --name app-redis --detach --restart unless-stopped --network kamal --log-opt max-size="10m" --publish 127.0.0.1:6379:6379 --env KAMAL_HOST="` + host + `" --env-file .kamal/apps/app/env/accessories/redis.env --volume $PWD/app-redis/data:/data --label service="app-redis" redis:latest on ` + host
}

func TestAccessoryBoot(t *testing.T) {
	output := accessoryRun(accessoryWorld(t), "boot", "mysql")
	assertContains(t, output, "docker login private.registry -u [REDACTED] -p [REDACTED] on 1.1.1.3")
	assertContains(t, output, mysqlRun)
	assertContains(t, output, "mkdir -p $PWD/app-mysql/data")
	assertContains(t, output, "to app-mysql/etc/mysql/my.cnf")
}

func TestAccessoryBootAll(t *testing.T) {
	output := accessoryRun(accessoryWorld(t), "boot", "all")
	for _, host := range []string{"1.1.1.3", "1.1.1.1", "1.1.1.2"} {
		assertContains(t, output, "docker login private.registry -u [REDACTED] -p [REDACTED] on "+host)
		assertMatches(t, output, `docker network create kamal.*on `+strings.ReplaceAll(host, ".", `\.`))
	}
	assertContains(t, output, "docker login other.registry -u [REDACTED] -p [REDACTED] on 1.1.1.3")
	assertContains(t, output, mysqlRun)
	assertContains(t, output, redisRun("1.1.1.1"))
	assertContains(t, output, redisRun("1.1.1.2"))
	assertContains(t, output, `docker run --name custom-box --detach --restart unless-stopped --network kamal --log-opt max-size="10m" --env KAMAL_HOST="1.1.1.3" --env-file .kamal/apps/app/env/accessories/busybox.env --label service="custom-box" other.registry/busybox:latest on 1.1.1.3`)
}

func TestAccessoryUploadAndDirectories(t *testing.T) {
	output := accessoryRun(accessoryWorld(t), "upload", "mysql")
	assertContains(t, output, "mkdir -p app-mysql/etc/mysql")
	assertContains(t, output, "test/fixtures/files/my.cnf to app-mysql/etc/mysql/my.cnf")
	assertContains(t, output, "chmod 755 app-mysql/etc/mysql/my.cnf")
	assertContains(t, accessoryRun(accessoryWorld(t), "directories", "mysql"), "mkdir -p $PWD/app-mysql/data")
}

func TestAccessoryReboot(t *testing.T) {
	output := accessoryRun(accessoryWorld(t), "reboot", "mysql")
	for _, want := range []string{"docker login private.registry", "docker image pull private.registry/mysql:5.7", "docker container stop app-mysql",
		"docker container prune --force --filter label=service=app-mysql", mysqlRun} {
		assertContains(t, output, want)
	}
}

func TestAccessoryRebootAll(t *testing.T) {
	output := accessoryRun(accessoryWorld(t), "reboot", "all")
	for _, name := range []string{"app-mysql", "app-redis", "custom-box"} {
		assertContains(t, output, "docker container stop "+name)
		assertContains(t, output, "docker container prune --force --filter label=service="+name)
	}
}

func TestAccessoryStartStopRestart(t *testing.T) {
	assertContains(t, accessoryRun(accessoryWorld(t), "start", "mysql"), "docker container start app-mysql")
	assertContains(t, accessoryRun(accessoryWorld(t), "stop", "mysql"), "docker container stop app-mysql")
	restart := accessoryRun(accessoryWorld(t), "restart", "mysql")
	assertContains(t, restart, "docker container stop app-mysql")
	assertContains(t, restart, "docker container start app-mysql")
}

func TestAccessoryDetails(t *testing.T) {
	output := accessoryRun(accessoryWorld(t), "details", "mysql")
	assertContains(t, output, "docker ps --filter label=service=app-mysql")
	assertContains(t, output, "Accessory mysql Host: 1.1.1.3")
	all := accessoryRun(accessoryWorld(t), "details", "all")
	for _, want := range []string{"Accessory mysql Host: 1.1.1.3", "Accessory redis Host: 1.1.1.2", "docker ps --filter label=service=app-mysql", "docker ps --filter label=service=app-redis"} {
		assertContains(t, all, want)
	}
}

func TestAccessoryDetailsWithNonExistentAccessory(t *testing.T) {
	w := accessoryWorld(t)
	accessoryRun(w, "details", "hello")
	if got := strings.TrimSpace(w.errOut.String()); got != "No accessory by the name of 'hello' (options: mysql, redis, and busybox)" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestAccessoryExec(t *testing.T) {
	output := accessoryRun(accessoryWorld(t), "exec", "mysql", "mysql -v")
	assertContains(t, output, "docker login private.registry -u [REDACTED] -p [REDACTED]")
	assertContains(t, output, "Launching command from new container")
	assertContains(t, output, "mysql -v")
	reuse := accessoryRun(accessoryWorld(t), "exec", "mysql", "--reuse", "mysql -v")
	assertContains(t, reuse, "Launching command from existing container")
	assertContains(t, reuse, "docker exec app-mysql mysql -v")
}

func TestAccessoryLogs(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{nil, "docker logs app-mysql  --tail 100 --timestamps 2>&1"},
		{[]string{"--grep", "hey"}, "docker logs app-mysql --timestamps 2>&1 | grep 'hey'"},
		{[]string{"--grep", "hey", "--grep-options", "-C 2"}, "docker logs app-mysql --timestamps 2>&1 | grep 'hey' -C 2"},
	}
	for _, c := range cases {
		assertContains(t, accessoryRun(accessoryWorld(t), append([]string{"logs", "mysql"}, c.args...)...), c.want)
	}
}

func TestAccessoryLogsWithFollow(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"mysql", "--follow"}, "ssh -t root@1.1.1.3 -p 22 'docker logs app-mysql --timestamps --tail 10 --follow 2>&1'"},
		{[]string{"mysql", "--follow", "--since", "5m"}, "ssh -t root@1.1.1.3 -p 22 'docker logs app-mysql --timestamps --since 5m --follow 2>&1'"},
		{[]string{"mysql", "--follow", "--lines", "123"}, "ssh -t root@1.1.1.3 -p 22 'docker logs app-mysql --timestamps --tail 123 --follow 2>&1'"},
		{[]string{"mysql", "--follow", "--grep", "hey"}, `ssh -t root@1.1.1.3 -p 22 'docker logs app-mysql --timestamps --follow 2>&1 | grep "hey"'`},
		{[]string{"mysql", "--follow", "--grep", "hey", "--grep-options", "-C 2"}, `ssh -t root@1.1.1.3 -p 22 'docker logs app-mysql --timestamps --follow 2>&1 | grep "hey" -C 2'`},
		{[]string{"redis", "--follow"}, "ssh -t root@1.1.1.1 -p 22 'docker logs app-redis --timestamps --tail 10 --follow 2>&1'"},
		{[]string{"redis", "--follow", "--hosts", "1.1.1.2"}, "ssh -t root@1.1.1.2 -p 22 'docker logs app-redis --timestamps --tail 10 --follow 2>&1'"},
	}
	for _, c := range cases {
		w := accessoryWorld(t)
		assertContains(t, accessoryRun(w, append([]string{"logs"}, c.args...)...), c.want)
		if len(w.execs) != 1 || w.execs[0] != c.want {
			t.Errorf("%v: exec = %v", c.args, w.execs)
		}
	}
}

func TestAccessoryLogsWithFollowDoesNothingWhenAccessoryDoesNotRunOnTheHosts(t *testing.T) {
	w := accessoryWorld(t)
	accessoryRun(w, "logs", "mysql", "--follow", "--hosts", "1.1.1.1")
	if len(w.execs) != 0 {
		t.Fatalf("exec = %v", w.execs)
	}
}

func TestAccessoryRemove(t *testing.T) {
	output := accessoryRun(accessoryWorld(t), "remove", "mysql", "-y")
	for _, want := range []string{"docker container stop app-mysql", "docker container prune --force --filter label=service=app-mysql",
		"docker image rm --force private.registry/mysql:5.7", "rm -rf app-mysql"} {
		assertContains(t, output, want)
	}
	all := accessoryRun(accessoryWorld(t), "remove", "all", "-y")
	for _, name := range []string{"app-mysql", "app-redis", "custom-box"} {
		assertContains(t, all, "rm -rf "+name)
	}
}

func TestAccessoryHiddenCommands(t *testing.T) {
	cases := map[string]string{
		"remove_container": "docker container prune --force --filter label=service=app-mysql",
		"pull_image":       "docker image pull private.registry/mysql:5.7", "remove_image": "docker image rm --force private.registry/mysql:5.7",
		"remove_service_directory": "rm -rf app-mysql",
	}
	for command, want := range cases {
		assertContains(t, accessoryRun(accessoryWorld(t), command, "mysql"), want)
	}
}

func TestAccessoryHostsParam(t *testing.T) {
	for hosts, excluded := range map[string]string{"1.1.1.1": "1.1.1.2", "1.1.1.1,1.1.1.3": "1.1.1.3"} {
		output := accessoryRun(accessoryWorld(t), "boot", "redis", "--hosts", hosts)
		assertContains(t, output, "docker login private.registry -u [REDACTED] -p [REDACTED] on 1.1.1.1")
		assertNotContains(t, output, "docker login private.registry -u [REDACTED] -p [REDACTED] on "+excluded)
		assertContains(t, output, redisRun("1.1.1.1"))
		assertNotContains(t, output, redisRun(excluded))
	}
}

func TestAccessoryUpgrade(t *testing.T) {
	output := accessoryRun(accessoryWorld(t), "upgrade", "-y", "all")
	for _, want := range []string{"Upgrading all accessories on 1.1.1.3,1.1.1.1,1.1.1.2...", "docker network create kamal on 1.1.1.3",
		"docker container stop app-mysql on 1.1.1.3", mysqlRun, "Upgraded all accessories on 1.1.1.3,1.1.1.1,1.1.1.2..."} {
		assertContains(t, output, want)
	}
	rolling := accessoryRun(accessoryWorld(t), "upgrade", "--rolling", "-y", "all")
	for _, want := range []string{"Upgrading all accessories on 1.1.1.3...", "docker network create kamal on 1.1.1.3",
		"docker container stop app-mysql on 1.1.1.3", mysqlRun, "Upgraded all accessories on 1.1.1.3"} {
		assertContains(t, rolling, want)
	}
}

func TestAccessoryBootWithRoleFilter(t *testing.T) {
	web := accessoryRun(accessoryWorld(t), "boot", "redis", "-r", "web")
	assertContains(t, web, redisRun("1.1.1.1"))
	assertContains(t, web, redisRun("1.1.1.2"))
	assertNotContains(t, accessoryRun(accessoryWorld(t), "boot", "redis", "-r", "workers"), "docker run")
}
