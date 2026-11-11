package commands

// Generated from test/commands/app_test.rb at Kamal 2.12.0 by the port's test
// generator. Each test keeps the Ruby test name; edit with care.

import "testing"

// run (test/commands/app_test.rb:14)
func TestAppRun(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `docker run --detach --restart unless-stopped --name app-web-999 --network kamal --env KAMAL_CONTAINER_NAME="app-web-999" --env KAMAL_VERSION="999" --env KAMAL_HOST="1.1.1.1" --env-file .kamal/apps/app/env/roles/web.env --log-opt max-size="10m" --label service="app" --label role="web" --label destination dhh/app:999`, c.app(t, "web", "1.1.1.1").Run("").String())
}

// run with destination (test/commands/app_test.rb:20)
func TestAppRunWithDestination(t *testing.T) {
	c := newAppCase(t)
	c.destination = "staging"
	assertText(t, `docker run --detach --restart unless-stopped --name app-web-staging-999 --network kamal --env KAMAL_CONTAINER_NAME="app-web-staging-999" --env KAMAL_VERSION="999" --env KAMAL_HOST="1.1.1.1" --env KAMAL_DESTINATION="staging" --env-file .kamal/apps/app-staging/env/roles/web.env --log-opt max-size="10m" --label service="app" --label role="web" --label destination="staging" dhh/app:999`, c.app(t, "web", "1.1.1.1").Run("").String())
}

// run with hostname (test/commands/app_test.rb:27)
func TestAppRunWithHostname(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `docker run --detach --restart unless-stopped --name app-web-999 --network kamal --hostname myhost --env KAMAL_CONTAINER_NAME="app-web-999" --env KAMAL_VERSION="999" --env KAMAL_HOST="1.1.1.1" --env-file .kamal/apps/app/env/roles/web.env --log-opt max-size="10m" --label service="app" --label role="web" --label destination dhh/app:999`, c.app(t, "web", "1.1.1.1").Run("myhost").String())
}

// run with volumes (test/commands/app_test.rb:33)
func TestAppRunWithVolumes(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `["/local/path:/container/path"]`, "volumes")
	assertText(t, `docker run --detach --restart unless-stopped --name app-web-999 --network kamal --env KAMAL_CONTAINER_NAME="app-web-999" --env KAMAL_VERSION="999" --env KAMAL_HOST="1.1.1.1" --env-file .kamal/apps/app/env/roles/web.env --log-opt max-size="10m" --volume /local/path:/container/path --label service="app" --label role="web" --label destination dhh/app:999`, c.app(t, "web", "1.1.1.1").Run("").String())
}

// run with custom options (test/commands/app_test.rb:41)
func TestAppRunWithCustomOptions(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"web": ["1.1.1.1"], "jobs": {"hosts": ["1.1.1.2"], "cmd": "bin/jobs", "options": {"mount": "somewhere", "cap-add": true}}}`, "servers")
	assertText(t, `docker run --detach --restart unless-stopped --name app-jobs-999 --network kamal --env KAMAL_CONTAINER_NAME="app-jobs-999" --env KAMAL_VERSION="999" --env KAMAL_HOST="1.1.1.2" --env-file .kamal/apps/app/env/roles/jobs.env --log-opt max-size="10m" --label service="app" --label role="jobs" --label destination --mount "somewhere" --cap-add dhh/app:999 bin/jobs`, c.app(t, "jobs", "1.1.1.2").Run("").String())
}

// run with custom restart policy (test/commands/app_test.rb:48)
func TestAppRunWithCustomRestartPolicy(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"web": {"hosts": ["1.1.1.1"], "options": {"restart": "on-failure"}}}`, "servers")
	assertText(t, `docker run --detach --restart on-failure --name app-web-999 --network kamal --env KAMAL_CONTAINER_NAME="app-web-999" --env KAMAL_VERSION="999" --env KAMAL_HOST="1.1.1.1" --env-file .kamal/apps/app/env/roles/web.env --log-opt max-size="10m" --label service="app" --label role="web" --label destination dhh/app:999`, c.app(t, "web", "1.1.1.1").Run("").String())
}

// run with logging config (test/commands/app_test.rb:56)
func TestAppRunWithLoggingConfig(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"driver": "local", "options": {"max-size": "100m", "max-file": "3"}}`, "logging")
	assertText(t, `docker run --detach --restart unless-stopped --name app-web-999 --network kamal --env KAMAL_CONTAINER_NAME="app-web-999" --env KAMAL_VERSION="999" --env KAMAL_HOST="1.1.1.1" --env-file .kamal/apps/app/env/roles/web.env --log-driver "local" --log-opt max-size="100m" --log-opt max-file="3" --label service="app" --label role="web" --label destination dhh/app:999`, c.app(t, "web", "1.1.1.1").Run("").String())
}

// run with role logging config (test/commands/app_test.rb:64)
func TestAppRunWithRoleLoggingConfig(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"driver": "local", "options": {"max-size": "10m", "max-file": "3"}}`, "logging")
	c.set(t, `{"web": {"hosts": ["1.1.1.1"], "logging": {"driver": "local", "options": {"max-size": "100m"}}}}`, "servers")
	assertText(t, `docker run --detach --restart unless-stopped --name app-web-999 --network kamal --env KAMAL_CONTAINER_NAME="app-web-999" --env KAMAL_VERSION="999" --env KAMAL_HOST="1.1.1.1" --env-file .kamal/apps/app/env/roles/web.env --log-driver "local" --log-opt max-size="100m" --log-opt max-file="3" --label service="app" --label role="web" --label destination dhh/app:999`, c.app(t, "web", "1.1.1.1").Run("").String())
}

// run with tags (test/commands/app_test.rb:73)
func TestAppRunWithTags(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `[{"1.1.1.1": "tag1"}]`, "servers")
	c.set(t, `{"tag1": {"ENV1": "value1"}}`, "env", "tags")
	assertText(t, `docker run --detach --restart unless-stopped --name app-web-999 --network kamal --env KAMAL_CONTAINER_NAME="app-web-999" --env KAMAL_VERSION="999" --env KAMAL_HOST="1.1.1.1" --env ENV1="value1" --env-file .kamal/apps/app/env/roles/web.env --log-opt max-size="10m" --label service="app" --label role="web" --label destination dhh/app:999`, c.app(t, "web", "1.1.1.1").Run("").String())
}

// start (test/commands/app_test.rb:82)
func TestAppStart(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `docker start app-web-999`, c.app(t, "web", "1.1.1.1").Start().String())
}

// start with destination (test/commands/app_test.rb:88)
func TestAppStartWithDestination(t *testing.T) {
	c := newAppCase(t)
	c.destination = "staging"
	assertText(t, `docker start app-web-staging-999`, c.app(t, "web", "1.1.1.1").Start().String())
}

// stop (test/commands/app_test.rb:95)
func TestAppStop(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `sh -c 'docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''{{.ID}}'\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting' | head -1 | xargs docker stop`, c.app(t, "web", "1.1.1.1").Stop("").String())
}

// stop with custom drain timeout (test/commands/app_test.rb:101)
func TestAppStopWithCustomDrainTimeout(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `20`, "drain_timeout")
	assertText(t, `sh -c 'docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''{{.ID}}'\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting' | head -1 | xargs docker stop`, c.app(t, "web", "1.1.1.1").Stop("").String())
	assertText(t, `sh -c 'docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=workers --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''{{.ID}}'\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=workers --filter status=running --filter status=restarting' | head -1 | xargs docker stop -t 20`, c.app(t, "workers", "1.1.1.1").Stop("").String())
}

// stop with stop_timeout (test/commands/app_test.rb:112)
func TestAppStopWithStopTimeout(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"web": {"hosts": ["1.1.1.1"], "stop_timeout": 30}, "workers": {"hosts": ["1.1.1.2"], "cmd": "bin/jobs", "stop_timeout": 60}}`, "servers")
	assertText(t, `sh -c 'docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''{{.ID}}'\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting' | head -1 | xargs docker stop -t 30`, c.app(t, "web", "1.1.1.1").Stop("").String())
	assertText(t, `sh -c 'docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=workers --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''{{.ID}}'\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=workers --filter status=running --filter status=restarting' | head -1 | xargs docker stop -t 60`, c.app(t, "workers", "1.1.1.1").Stop("").String())
}

// stop with version (test/commands/app_test.rb:123)
func TestAppStopWithVersion(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `docker container ls --all --filter 'name=^app-web-123$' --quiet | xargs docker stop`, c.app(t, "web", "1.1.1.1").Stop("123").String())
}

// info (test/commands/app_test.rb:129)
func TestAppInfo(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `docker ps --filter label=service=app --filter label=destination= --filter label=role=web`, c.app(t, "web", "1.1.1.1").Info().String())
}

// info with destination (test/commands/app_test.rb:135)
func TestAppInfoWithDestination(t *testing.T) {
	c := newAppCase(t)
	c.destination = "staging"
	assertText(t, `docker ps --filter label=service=app --filter label=destination=staging --filter label=role=web`, c.app(t, "web", "1.1.1.1").Info().String())
}

// deploy (test/commands/app_test.rb:142)
func TestAppDeploy(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `docker exec kamal-proxy kamal-proxy deploy app-web --target="172.1.0.2:80" --deploy-timeout="30s" --drain-timeout="30s" --buffer-requests --buffer-responses --log-request-header="Cache-Control" --log-request-header="Last-Modified" --log-request-header="User-Agent"`, c.app(t, "web", "1.1.1.1").Deploy("172.1.0.2").String())
}

// deploy with SSL (test/commands/app_test.rb:148)
func TestAppDeployWithSSL(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"ssl": true, "host": "example.com"}`, "proxy")
	assertText(t, `docker exec kamal-proxy kamal-proxy deploy app-web --target="172.1.0.2:80" --host="example.com" --tls --deploy-timeout="30s" --drain-timeout="30s" --buffer-requests --buffer-responses --log-request-header="Cache-Control" --log-request-header="Last-Modified" --log-request-header="User-Agent"`, c.app(t, "web", "1.1.1.1").Deploy("172.1.0.2").String())
}

// deploy with SSL targeting multiple hosts (test/commands/app_test.rb:156)
func TestAppDeployWithSSLTargetingMultipleHosts(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"ssl": true, "hosts": ["example.com", "anotherexample.com"]}`, "proxy")
	assertText(t, `docker exec kamal-proxy kamal-proxy deploy app-web --target="172.1.0.2:80" --host="example.com" --host="anotherexample.com" --tls --deploy-timeout="30s" --drain-timeout="30s" --buffer-requests --buffer-responses --log-request-header="Cache-Control" --log-request-header="Last-Modified" --log-request-header="User-Agent"`, c.app(t, "web", "1.1.1.1").Deploy("172.1.0.2").String())
}

// deploy with SSL false (test/commands/app_test.rb:164)
func TestAppDeployWithSSLFalse(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"ssl": false}`, "proxy")
	assertText(t, `docker exec kamal-proxy kamal-proxy deploy app-web --target="172.1.0.2:80" --deploy-timeout="30s" --drain-timeout="30s" --buffer-requests --buffer-responses --log-request-header="Cache-Control" --log-request-header="Last-Modified" --log-request-header="User-Agent"`, c.app(t, "web", "1.1.1.1").Deploy("172.1.0.2").String())
}

// remove (test/commands/app_test.rb:172)
func TestAppRemove(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `docker exec kamal-proxy kamal-proxy remove app-web`, c.app(t, "web", "1.1.1.1").Remove().String())
}

// logs (test/commands/app_test.rb:178)
func TestAppLogs(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `sh -c 'docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''{{.ID}}'\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting' | head -1 | xargs docker logs --timestamps 2>&1`, c.app(t, "web", "1.1.1.1").Logs(LogOptions{Timestamps: true}).String())
}

// logs with container_id (test/commands/app_test.rb:184)
func TestAppLogsWithContainerId(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `sh -c 'echo C137' | xargs docker logs --timestamps 2>&1`, c.app(t, "web", "1.1.1.1").Logs(LogOptions{Timestamps: true, ContainerID: "C137"}).String())
}

// logs with since (test/commands/app_test.rb:190)
func TestAppLogsWithSince(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `sh -c 'docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''{{.ID}}'\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting' | head -1 | xargs docker logs --timestamps --since 5m 2>&1`, c.app(t, "web", "1.1.1.1").Logs(LogOptions{Timestamps: true, Since: "5m"}).String())
}

// logs with lines (test/commands/app_test.rb:196)
func TestAppLogsWithLines(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `sh -c 'docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''{{.ID}}'\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting' | head -1 | xargs docker logs --timestamps --tail 100 2>&1`, c.app(t, "web", "1.1.1.1").Logs(LogOptions{Timestamps: true, Lines: "100"}).String())
}

// logs with since and lines (test/commands/app_test.rb:202)
func TestAppLogsWithSinceAndLines(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `sh -c 'docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''{{.ID}}'\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting' | head -1 | xargs docker logs --timestamps --since 5m --tail 100 2>&1`, c.app(t, "web", "1.1.1.1").Logs(LogOptions{Timestamps: true, Since: "5m", Lines: "100"}).String())
}

// logs with grep (test/commands/app_test.rb:208)
func TestAppLogsWithGrep(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `sh -c 'docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''{{.ID}}'\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting' | head -1 | xargs docker logs --timestamps 2>&1 | grep 'my-id'`, c.app(t, "web", "1.1.1.1").Logs(LogOptions{Timestamps: true, Grep: "my-id"}).String())
}

// logs with grep and grep options (test/commands/app_test.rb:214)
func TestAppLogsWithGrepAndGrepOptions(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `sh -c 'docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''{{.ID}}'\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting' | head -1 | xargs docker logs --timestamps 2>&1 | grep 'my-id' -C 2`, c.app(t, "web", "1.1.1.1").Logs(LogOptions{Timestamps: true, Grep: "my-id", GrepOptions: "-C 2"}).String())
}

// logs with since, grep and grep options (test/commands/app_test.rb:220)
func TestAppLogsWithSinceGrepAndGrepOptions(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `sh -c 'docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''{{.ID}}'\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting' | head -1 | xargs docker logs --timestamps --since 5m 2>&1 | grep 'my-id' -C 2`, c.app(t, "web", "1.1.1.1").Logs(LogOptions{Timestamps: true, Since: "5m", Grep: "my-id", GrepOptions: "-C 2"}).String())
}

// logs with since and grep (test/commands/app_test.rb:226)
func TestAppLogsWithSinceAndGrep(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `sh -c 'docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''{{.ID}}'\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting' | head -1 | xargs docker logs --timestamps --since 5m 2>&1 | grep 'my-id'`, c.app(t, "web", "1.1.1.1").Logs(LogOptions{Timestamps: true, Since: "5m", Grep: "my-id"}).String())
}

// follow logs (test/commands/app_test.rb:232)
func TestAppFollowLogs(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `ssh -t root@app-1 -p 22 'sh -c '\''docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''\'\'''\''{{.ID}}'\''\'\'''\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting'\'' | head -1 | xargs docker logs --timestamps --follow 2>&1'`, c.app(t, "web", "1.1.1.1").FollowLogs("app-1", LogOptions{Timestamps: true}))
	assertText(t, `ssh -t root@app-1 -p 22 'sh -c '\''docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''\'\'''\''{{.ID}}'\''\'\'''\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting'\'' | head -1 | xargs docker logs --timestamps --follow 2>&1 | grep "Completed"'`, c.app(t, "web", "1.1.1.1").FollowLogs("app-1", LogOptions{Timestamps: true, Grep: "Completed"}))
	assertText(t, `ssh -t root@app-1 -p 22 'sh -c '\''echo ID321'\'' | xargs docker logs --timestamps --follow 2>&1'`, c.app(t, "web", "1.1.1.1").FollowLogs("app-1", LogOptions{Timestamps: true, ContainerID: "ID321"}))
	assertText(t, `ssh -t root@app-1 -p 22 'sh -c '\''docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''\'\'''\''{{.ID}}'\''\'\'''\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting'\'' | head -1 | xargs docker logs --timestamps --tail 123 --follow 2>&1'`, c.app(t, "web", "1.1.1.1").FollowLogs("app-1", LogOptions{Timestamps: true, Lines: "123"}))
	assertText(t, `ssh -t root@app-1 -p 22 'sh -c '\''docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''\'\'''\''{{.ID}}'\''\'\'''\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting'\'' | head -1 | xargs docker logs --timestamps --tail 123 --follow 2>&1 | grep "Completed"'`, c.app(t, "web", "1.1.1.1").FollowLogs("app-1", LogOptions{Timestamps: true, Lines: "123", Grep: "Completed"}))
	assertText(t, `ssh -t root@app-1 -p 22 'sh -c '\''docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''\'\'''\''{{.ID}}'\''\'\'''\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting'\'' | head -1 | xargs docker logs --tail 123 --follow 2>&1 | grep "Completed"'`, c.app(t, "web", "1.1.1.1").FollowLogs("app-1", LogOptions{Lines: "123", Grep: "Completed"}))
	assertText(t, `ssh -t root@app-1 -p 22 'sh -c '\''docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''\'\'''\''{{.ID}}'\''\'\'''\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting'\'' | head -1 | xargs docker logs --timestamps --since 5m --follow 2>&1'`, c.app(t, "web", "1.1.1.1").FollowLogs("app-1", LogOptions{Timestamps: true, Since: "5m"}))
	assertText(t, `ssh -t root@app-1 -p 22 'sh -c '\''docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''\'\'''\''{{.ID}}'\''\'\'''\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting'\'' | head -1 | xargs docker logs --timestamps --since 5m --tail 123 --follow 2>&1'`, c.app(t, "web", "1.1.1.1").FollowLogs("app-1", LogOptions{Timestamps: true, Since: "5m", Lines: "123"}))
	assertText(t, `ssh -t root@app-1 -p 22 'sh -c '\''docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''\'\'''\''{{.ID}}'\''\'\'''\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting'\'' | head -1 | xargs docker logs --timestamps --since 5m --follow 2>&1 | grep "Completed"'`, c.app(t, "web", "1.1.1.1").FollowLogs("app-1", LogOptions{Timestamps: true, Since: "5m", Grep: "Completed"}))
}

// follow logs with ssh keys (test/commands/app_test.rb:270)
func TestAppFollowLogsWithSshKeys(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"keys": ["path_to_key.pem"]}`, "ssh")
	assertText(t, `ssh -i path_to_key.pem -t root@app-1 -p 22 'sh -c '\''docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''\'\'''\''{{.ID}}'\''\'\'''\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting'\'' | head -1 | xargs docker logs --timestamps --follow 2>&1'`, c.app(t, "web", "1.1.1.1").FollowLogs("app-1", LogOptions{Timestamps: true}))
}

// follow logs with ssh proxy_command (test/commands/app_test.rb:277)
func TestAppFollowLogsWithSshProxyCommand(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"proxy_command": "ssh -W %h:%p user@proxy-server"}`, "ssh")
	assertText(t, `ssh -o ProxyCommand='ssh -W %h:%p user@proxy-server' -t root@app-1 -p 22 'sh -c '\''docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''\'\'''\''{{.ID}}'\''\'\'''\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting'\'' | head -1 | xargs docker logs --timestamps --follow 2>&1'`, c.app(t, "web", "1.1.1.1").FollowLogs("app-1", LogOptions{Timestamps: true}))
}

// follow logs with ssh config file (test/commands/app_test.rb:284)
func TestAppFollowLogsWithSshConfigFile(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"config": "~/.ssh/custom_config"}`, "ssh")
	assertText(t, `ssh -F ~/.ssh/custom_config -t root@app-1 -p 22 'sh -c '\''docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''\'\'''\''{{.ID}}'\''\'\'''\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting'\'' | head -1 | xargs docker logs --timestamps --follow 2>&1'`, c.app(t, "web", "1.1.1.1").FollowLogs("app-1", LogOptions{Timestamps: true}))
}

// execute in new container (test/commands/app_test.rb:291)
func TestAppExecuteInNewContainer(t *testing.T) {
	c := newAppCase(t)
	assertMatch(t, `docker run --rm --name app-web-exec-999-[0-9a-f]{6} --network kamal --env-file .kamal/apps/app/env/roles/web.env --log-opt max-size="10m" dhh/app:999 bin/rails db:setup`, c.app(t, "web", "1.1.1.1").ExecuteInNewContainer([]string{"bin/rails", "db:setup"}, false, false, c.env(t, `{}`)).String())
}

// execute in new container with logging (test/commands/app_test.rb:297)
func TestAppExecuteInNewContainerWithLogging(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"driver": "local", "options": {"max-size": "100m", "max-file": "3"}}`, "logging")
	assertMatch(t, `docker run --rm --name app-web-exec-999-[0-9a-f]{6} --network kamal --env-file .kamal/apps/app/env/roles/web.env --log-driver "local" --log-opt max-size="100m" --log-opt max-file="3" dhh/app:999 bin/rails db:setup`, c.app(t, "web", "1.1.1.1").ExecuteInNewContainer([]string{"bin/rails", "db:setup"}, false, false, c.env(t, `{}`)).String())
}

// execute in new container with env (test/commands/app_test.rb:305)
func TestAppExecuteInNewContainerWithEnv(t *testing.T) {
	c := newAppCase(t)
	assertMatch(t, `docker run --rm --name app-web-exec-999-[0-9a-f]{6} --network kamal --env-file .kamal/apps/app/env/roles/web.env --env foo="bar" --log-opt max-size="10m" dhh/app:999 bin/rails db:setup`, c.app(t, "web", "1.1.1.1").ExecuteInNewContainer([]string{"bin/rails", "db:setup"}, false, false, c.env(t, `{"foo": "bar"}`)).String())
}

// execute in new detached container (test/commands/app_test.rb:311)
func TestAppExecuteInNewDetachedContainer(t *testing.T) {
	c := newAppCase(t)
	assertMatch(t, `docker run --detach --name app-web-exec-999-[0-9a-f]{6} --network kamal --env-file .kamal/apps/app/env/roles/web.env --log-opt max-size="10m" dhh/app:999 bin/rails db:setup`, c.app(t, "web", "1.1.1.1").ExecuteInNewContainer([]string{"bin/rails", "db:setup"}, false, true, c.env(t, `{}`)).String())
}

// execute in new container with tags (test/commands/app_test.rb:317)
func TestAppExecuteInNewContainerWithTags(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `[{"1.1.1.1": "tag1"}]`, "servers")
	c.set(t, `{"tag1": {"ENV1": "value1"}}`, "env", "tags")
	assertMatch(t, `docker run --rm --name app-web-exec-999-[0-9a-f]{6} --network kamal --env ENV1="value1" --env-file .kamal/apps/app/env/roles/web.env --log-opt max-size="10m" dhh/app:999 bin/rails db:setup`, c.app(t, "web", "1.1.1.1").ExecuteInNewContainer([]string{"bin/rails", "db:setup"}, false, false, c.env(t, `{}`)).String())
}

// execute in new container with custom options (test/commands/app_test.rb:326)
func TestAppExecuteInNewContainerWithCustomOptions(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"web": {"hosts": ["1.1.1.1"], "options": {"mount": "somewhere", "cap-add": true}}}`, "servers")
	assertMatch(t, `docker run --rm --name app-web-exec-999-[0-9a-f]{6} --network kamal --env-file .kamal/apps/app/env/roles/web.env --log-opt max-size="10m" --mount "somewhere" --cap-add dhh/app:999 bin/rails db:setup`, c.app(t, "web", "1.1.1.1").ExecuteInNewContainer([]string{"bin/rails", "db:setup"}, false, false, c.env(t, `{}`)).String())
}

// execute in new container excludes restart policy (test/commands/app_test.rb:333)
func TestAppExecuteInNewContainerExcludesRestartPolicy(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"web": {"hosts": ["1.1.1.1"], "options": {"restart": "on-failure", "mount": "somewhere"}}}`, "servers")
	command := c.app(t, "web", "1.1.1.1").ExecuteInNewContainer([]string{"bin/rails", "db:setup"}, false, false, c.env(t, `{}`)).String()
	assertMatch(t, `docker run --rm --name app-web-exec-999-[0-9a-f]{6} --network kamal --env-file .kamal/apps/app/env/roles/web.env --log-opt max-size="10m" --mount "somewhere" dhh/app:999 bin/rails db:setup`, command)
	assertNoMatch(t, `--restart`, command)
}

// execute in existing container (test/commands/app_test.rb:342)
func TestAppExecuteInExistingContainer(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `docker exec app-web-999 bin/rails db:setup`, c.app(t, "web", "1.1.1.1").ExecuteInExistingContainer([]string{"bin/rails", "db:setup"}, false, c.env(t, `{}`)).String())
}

// execute in existing container with env (test/commands/app_test.rb:348)
func TestAppExecuteInExistingContainerWithEnv(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `docker exec --env foo="bar" app-web-999 bin/rails db:setup`, c.app(t, "web", "1.1.1.1").ExecuteInExistingContainer([]string{"bin/rails", "db:setup"}, false, c.env(t, `{"foo": "bar"}`)).String())
}

// execute in new container over ssh (test/commands/app_test.rb:354)
func TestAppExecuteInNewContainerOverSsh(t *testing.T) {
	c := newAppCase(t)
	c.tty = true
	assertMatch(t, `docker run -it --rm --name app-web-exec-999-[0-9a-f]{6} --network kamal --env-file .kamal/apps/app/env/roles/web.env --log-opt max-size="10m" dhh/app:999 bin/rails c`, c.app(t, "web", "1.1.1.1").ExecuteInNewContainerOverSSH([]string{"bin/rails", "c"}, c.env(t, `{}`)))
}

// execute in new container over ssh with tags (test/commands/app_test.rb:359)
func TestAppExecuteInNewContainerOverSshWithTags(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `[{"1.1.1.1": "tag1"}]`, "servers")
	c.set(t, `{"tag1": {"ENV1": "value1"}}`, "env", "tags")
	c.tty = true
	assertMatch(t, `ssh -t root@1.1.1.1 -p 22 'docker run -it --rm --name app-web-exec-999-[0-9a-f]{6} --network kamal --env ENV1="value1" --env-file .kamal/apps/app/env/roles/web.env --log-opt max-size="10m" dhh/app:999 bin/rails c'`, c.app(t, "web", "1.1.1.1").ExecuteInNewContainerOverSSH([]string{"bin/rails", "c"}, c.env(t, `{}`)))
}

// execute in new container with custom options over ssh (test/commands/app_test.rb:367)
func TestAppExecuteInNewContainerWithCustomOptionsOverSsh(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"web": {"hosts": ["1.1.1.1"], "options": {"mount": "somewhere", "cap-add": true}}}`, "servers")
	c.tty = true
	assertMatch(t, `docker run -it --rm --name app-web-exec-999-[0-9a-f]{6} --network kamal --env-file .kamal/apps/app/env/roles/web.env --log-opt max-size=\"10m\" --mount \"somewhere\" --cap-add dhh/app:999 bin/rails c`, c.app(t, "web", "1.1.1.1").ExecuteInNewContainerOverSSH([]string{"bin/rails", "c"}, c.env(t, `{}`)))
}

// execute in existing container over ssh (test/commands/app_test.rb:373)
func TestAppExecuteInExistingContainerOverSsh(t *testing.T) {
	c := newAppCase(t)
	c.tty = true
	assertMatch(t, `docker exec -it app-web-999 bin/rails c`, c.app(t, "web", "1.1.1.1").ExecuteInExistingContainerOverSSH([]string{"bin/rails", "c"}, c.env(t, `{}`)))
}

// execute in existing container with piped input over ssh (test/commands/app_test.rb:378)
func TestAppExecuteInExistingContainerWithPipedInputOverSsh(t *testing.T) {
	c := newAppCase(t)
	c.tty = false
	assertMatch(t, `docker exec -i app-web-999 bin/rails c`, c.app(t, "web", "1.1.1.1").ExecuteInExistingContainerOverSSH([]string{"bin/rails", "c"}, c.env(t, `{}`)))
}

// run over ssh (test/commands/app_test.rb:383)
func TestAppRunOverSsh(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `ssh -t root@1.1.1.1 -p 22 'ls'`, c.app(t, "web", "1.1.1.1").RunOverSSH(w("ls"), "1.1.1.1"))
}

// run over ssh with custom user (test/commands/app_test.rb:387)
func TestAppRunOverSshWithCustomUser(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"user": "app"}`, "ssh")
	assertText(t, `ssh -t app@1.1.1.1 -p 22 'ls'`, c.app(t, "web", "1.1.1.1").RunOverSSH(w("ls"), "1.1.1.1"))
}

// run over ssh with custom port (test/commands/app_test.rb:392)
func TestAppRunOverSshWithCustomPort(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"port": "2222"}`, "ssh")
	assertText(t, `ssh -t root@1.1.1.1 -p 2222 'ls'`, c.app(t, "web", "1.1.1.1").RunOverSSH(w("ls"), "1.1.1.1"))
}

// run over ssh with proxy (test/commands/app_test.rb:397)
func TestAppRunOverSshWithProxy(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"proxy": "2.2.2.2"}`, "ssh")
	assertText(t, `ssh -J root@2.2.2.2 -t root@1.1.1.1 -p 22 'ls'`, c.app(t, "web", "1.1.1.1").RunOverSSH(w("ls"), "1.1.1.1"))
}

// run over ssh with proxy user (test/commands/app_test.rb:402)
func TestAppRunOverSshWithProxyUser(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"proxy": "app@2.2.2.2"}`, "ssh")
	assertText(t, `ssh -J app@2.2.2.2 -t root@1.1.1.1 -p 22 'ls'`, c.app(t, "web", "1.1.1.1").RunOverSSH(w("ls"), "1.1.1.1"))
}

// run over ssh with custom user with proxy (test/commands/app_test.rb:407)
func TestAppRunOverSshWithCustomUserWithProxy(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"user": "app", "proxy": "2.2.2.2"}`, "ssh")
	assertText(t, `ssh -J root@2.2.2.2 -t app@1.1.1.1 -p 22 'ls'`, c.app(t, "web", "1.1.1.1").RunOverSSH(w("ls"), "1.1.1.1"))
}

// run over ssh with keys config (test/commands/app_test.rb:412)
func TestAppRunOverSshWithKeysConfig(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"keys": ["path_to_key.pem"]}`, "ssh")
	assertText(t, `ssh -i path_to_key.pem -t root@1.1.1.1 -p 22 'ls'`, c.app(t, "web", "1.1.1.1").RunOverSSH(w("ls"), "1.1.1.1"))
}

// run over ssh with keys config with keys_only (test/commands/app_test.rb:417)
func TestAppRunOverSshWithKeysConfigWithKeysOnly(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"keys": ["path_to_key.pem"], "keys_only": true}`, "ssh")
	assertText(t, `ssh -i path_to_key.pem -o IdentitiesOnly=yes -t root@1.1.1.1 -p 22 'ls'`, c.app(t, "web", "1.1.1.1").RunOverSSH(w("ls"), "1.1.1.1"))
}

// run over ssh with proxy_command (test/commands/app_test.rb:422)
func TestAppRunOverSshWithProxyCommand(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"proxy_command": "ssh -W %h:%p user@proxy-server"}`, "ssh")
	assertText(t, `ssh -o ProxyCommand='ssh -W %h:%p user@proxy-server' -t root@1.1.1.1 -p 22 'ls'`, c.app(t, "web", "1.1.1.1").RunOverSSH(w("ls"), "1.1.1.1"))
}

// run over ssh with config file (test/commands/app_test.rb:427)
func TestAppRunOverSshWithConfigFile(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"config": "~/.ssh/custom_config"}`, "ssh")
	assertText(t, `ssh -F ~/.ssh/custom_config -t root@1.1.1.1 -p 22 'ls'`, c.app(t, "web", "1.1.1.1").RunOverSSH(w("ls"), "1.1.1.1"))
}

// run over ssh with multiple config files (test/commands/app_test.rb:432)
func TestAppRunOverSshWithMultipleConfigFiles(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"config": ["~/.ssh/config1", "~/.ssh/config2"]}`, "ssh")
	assertText(t, `ssh -F ~/.ssh/config1 -F ~/.ssh/config2 -t root@1.1.1.1 -p 22 'ls'`, c.app(t, "web", "1.1.1.1").RunOverSSH(w("ls"), "1.1.1.1"))
}

// run over ssh with config false (test/commands/app_test.rb:437)
func TestAppRunOverSshWithConfigFalse(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"config": false}`, "ssh")
	assertText(t, `ssh -F /dev/null -t root@1.1.1.1 -p 22 'ls'`, c.app(t, "web", "1.1.1.1").RunOverSSH(w("ls"), "1.1.1.1"))
}

// run over ssh with config true (test/commands/app_test.rb:442)
func TestAppRunOverSshWithConfigTrue(t *testing.T) {
	c := newAppCase(t)
	c.set(t, `{"config": true}`, "ssh")
	assertText(t, `ssh -t root@1.1.1.1 -p 22 'ls'`, c.app(t, "web", "1.1.1.1").RunOverSSH(w("ls"), "1.1.1.1"))
}

// current_running_container_id (test/commands/app_test.rb:447)
func TestAppCurrentRunningContainerId(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `sh -c 'docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''{{.ID}}'\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting' | head -1`, c.app(t, "web", "1.1.1.1").CurrentRunningContainerID().String())
}

// current_running_container_id with destination (test/commands/app_test.rb:453)
func TestAppCurrentRunningContainerIdWithDestination(t *testing.T) {
	c := newAppCase(t)
	c.destination = "staging"
	assertText(t, `sh -c 'docker ps --latest --quiet --filter label=service=app --filter label=destination=staging --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest-staging --format '\''{{.ID}}'\'') ; docker ps --latest --quiet --filter label=service=app --filter label=destination=staging --filter label=role=web --filter status=running --filter status=restarting' | head -1`, c.app(t, "web", "1.1.1.1").CurrentRunningContainerID().String())
}

// container_id_for (test/commands/app_test.rb:460)
func TestAppContainerIdFor(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `docker container ls --all --filter 'name=^app-999$' --quiet`, c.app(t, "web", "1.1.1.1").ContainerIDFor("app-999", false).String())
}

// current_running_version (test/commands/app_test.rb:466)
func TestAppCurrentRunningVersion(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `sh -c 'docker ps --latest --format '\''{{.Names}}'\'' --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --filter ancestor=$(docker image ls --filter reference=dhh/app:latest --format '\''{{.ID}}'\'') ; docker ps --latest --format '\''{{.Names}}'\'' --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting' | head -1 | while read line; do echo ${line#app-web-}; done`, c.app(t, "web", "1.1.1.1").CurrentRunningVersion().String())
}

// list_versions (test/commands/app_test.rb:472)
func TestAppListVersions(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `docker ps --filter label=service=app --filter label=destination= --filter label=role=web --format "{{.Names}}" | while read line; do echo ${line#app-web-}; done`, c.app(t, "web", "1.1.1.1").ListVersions([]string{}, nil).String())
	assertText(t, `docker ps --filter label=service=app --filter label=destination= --filter label=role=web --filter status=running --filter status=restarting --latest --format "{{.Names}}" | while read line; do echo ${line#app-web-}; done`, c.app(t, "web", "1.1.1.1").ListVersions([]string{"--latest"}, []string{"running", "restarting"}).String())
}

// list_containers (test/commands/app_test.rb:482)
func TestAppListContainers(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `docker container ls --all --filter label=service=app --filter label=destination= --filter label=role=web`, c.app(t, "web", "1.1.1.1").ListContainers().String())
}

// list_containers with destination (test/commands/app_test.rb:488)
func TestAppListContainersWithDestination(t *testing.T) {
	c := newAppCase(t)
	c.destination = "staging"
	assertText(t, `docker container ls --all --filter label=service=app --filter label=destination=staging --filter label=role=web`, c.app(t, "web", "1.1.1.1").ListContainers().String())
}

// list_container_names (test/commands/app_test.rb:495)
func TestAppListContainerNames(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `docker container ls --all --filter label=service=app --filter label=destination= --filter label=role=web --format '{{ .Names }}'`, c.app(t, "web", "1.1.1.1").ListContainerNames().String())
}

// remove_container (test/commands/app_test.rb:501)
func TestAppRemoveContainer(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `docker container ls --all --filter 'name=^app-web-999$' --quiet | xargs docker container rm`, c.app(t, "web", "1.1.1.1").RemoveContainer("999").String())
}

// remove_container with destination (test/commands/app_test.rb:507)
func TestAppRemoveContainerWithDestination(t *testing.T) {
	c := newAppCase(t)
	c.destination = "staging"
	assertText(t, `docker container ls --all --filter 'name=^app-web-staging-999$' --quiet | xargs docker container rm`, c.app(t, "web", "1.1.1.1").RemoveContainer("999").String())
}

// remove_containers (test/commands/app_test.rb:514)
func TestAppRemoveContainers(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `docker container prune --force --filter label=service=app --filter label=destination= --filter label=role=web`, c.app(t, "web", "1.1.1.1").RemoveContainers().String())
}

// remove_containers with destination (test/commands/app_test.rb:520)
func TestAppRemoveContainersWithDestination(t *testing.T) {
	c := newAppCase(t)
	c.destination = "staging"
	assertText(t, `docker container prune --force --filter label=service=app --filter label=destination=staging --filter label=role=web`, c.app(t, "web", "1.1.1.1").RemoveContainers().String())
}

// list_images (test/commands/app_test.rb:527)
func TestAppListImages(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `docker image ls dhh/app`, c.app(t, "web", "1.1.1.1").ListImages().String())
}

// remove_images (test/commands/app_test.rb:533)
func TestAppRemoveImages(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `docker image prune --all --force --filter label=service=app`, c.app(t, "web", "1.1.1.1").RemoveImages().String())
}

// remove_images with destination (test/commands/app_test.rb:539)
func TestAppRemoveImagesWithDestination(t *testing.T) {
	c := newAppCase(t)
	c.destination = "staging"
	assertText(t, `docker image prune --all --force --filter label=service=app`, c.app(t, "web", "1.1.1.1").RemoveImages().String())
}

// tag_latest_image (test/commands/app_test.rb:546)
func TestAppTagLatestImage(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `docker tag dhh/app:999 dhh/app:latest`, c.app(t, "web", "1.1.1.1").TagLatestImage().String())
}

// tag_latest_image with destination (test/commands/app_test.rb:552)
func TestAppTagLatestImageWithDestination(t *testing.T) {
	c := newAppCase(t)
	c.destination = "staging"
	assertText(t, `docker tag dhh/app:999 dhh/app:latest-staging`, c.app(t, "web", "1.1.1.1").TagLatestImage().String())
}

// extract assets (test/commands/app_test.rb:559)
func TestAppExtractAssets(t *testing.T) {
	c := newAppCase(t)
	assertWords(t, []string{"mkdir", "-p", ".kamal/apps/app/assets/extracted/web-999", "&&", "docker", "container", "rm", "app-web-assets", "2> /dev/null", "|| true", "&&", "docker", "container", "create", "--name", "app-web-assets", "dhh/app:999", "&&", "docker", "container", "cp", "-L", "app-web-assets:/public/assets/.", ".kamal/apps/app/assets/extracted/web-999", "&&", "docker", "container", "rm", "app-web-assets"}, c.extra(t, "asset_path", `"/public/assets"`).app(t, "web", "1.1.1.1").ExtractAssets())
}

// sync asset volumes (test/commands/app_test.rb:569)
func TestAppSyncAssetVolumes(t *testing.T) {
	c := newAppCase(t)
	assertWords(t, []string{"mkdir", "-p", ".kamal/apps/app/assets/volumes/web-999", ";", "cp", "-rnT", ".kamal/apps/app/assets/extracted/web-999", ".kamal/apps/app/assets/volumes/web-999"}, c.extra(t, "asset_path", `"/public/assets"`).app(t, "web", "1.1.1.1").SyncAssetVolumes(""))
	assertWords(t, []string{"mkdir", "-p", ".kamal/apps/app/assets/volumes/web-999", ";", "cp", "-rnT", ".kamal/apps/app/assets/extracted/web-999", ".kamal/apps/app/assets/volumes/web-999", ";", "cp", "-rnT", ".kamal/apps/app/assets/extracted/web-999", ".kamal/apps/app/assets/volumes/web-998", "|| true", ";", "cp", "-rnT", ".kamal/apps/app/assets/extracted/web-998", ".kamal/apps/app/assets/volumes/web-999", "|| true"}, c.extra(t, "asset_path", `"/public/assets"`).app(t, "web", "1.1.1.1").SyncAssetVolumes("998"))
}

// clean up assets (test/commands/app_test.rb:583)
func TestAppCleanUpAssets(t *testing.T) {
	c := newAppCase(t)
	assertWords(t, []string{"find", ".kamal/apps/app/assets/extracted", "-maxdepth 1", "-name", "'web-*'", "!", "-name", "web-999", "-exec rm -rf \"{}\" +", ";", "find", ".kamal/apps/app/assets/volumes", "-maxdepth 1", "-name", "'web-*'", "!", "-name", "web-999", "-exec rm -rf \"{}\" +"}, c.extra(t, "asset_path", `"/public/assets"`).app(t, "web", "1.1.1.1").CleanUpAssets())
}

// live (test/commands/app_test.rb:590)
func TestAppLive(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `docker exec kamal-proxy kamal-proxy resume app-web`, c.app(t, "web", "1.1.1.1").Live().String())
}

// maintenance (test/commands/app_test.rb:596)
func TestAppMaintenance(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `docker exec kamal-proxy kamal-proxy stop app-web`, c.app(t, "web", "1.1.1.1").Maintenance(nil, "").String())
}

// maintenance with options (test/commands/app_test.rb:602)
func TestAppMaintenanceWithOptions(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `docker exec kamal-proxy kamal-proxy stop app-web --drain-timeout="10s" --message="Hi"`, c.app(t, "web", "1.1.1.1").Maintenance(intValue(10), "Hi").String())
}

// remove_proxy_app_directory (test/commands/app_test.rb:608)
func TestAppRemoveProxyAppDirectory(t *testing.T) {
	c := newAppCase(t)
	assertText(t, `rm -r .kamal/proxy/apps-config/app`, c.app(t, "web", "1.1.1.1").RemoveProxyAppDirectory().String())
}
