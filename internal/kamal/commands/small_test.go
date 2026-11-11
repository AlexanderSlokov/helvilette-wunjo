package commands

// Ports test/commands/docker_test.rb, server_test.rb, registry_test.rb,
// prune_test.rb, lock_test.rb, auditor_test.rb, and hook_test.rb at Kamal 2.12.0.

import (
	"encoding/base64"
	"regexp"
	"strings"
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/tags"
)

func TestDockerCommands(t *testing.T) {
	d := Docker{newCase(t, simpleDeploy).base(t)}
	cases := []struct {
		name, want string
		line       Line
	}{
		{"install", `sh -c 'curl -fsSL https://get.docker.com || wget -O - https://get.docker.com || echo "exit 1"' | sh`, d.Install()},
		{"installed?", "docker -v", d.Installed()},
		{"running?", "docker version", d.Running()},
		{"superuser?", `[ "${EUID:-$(id -u)}" -eq 0 ] || sudo -nl usermod >/dev/null`, d.Superuser()},
		{"root?", `[ "${EUID:-$(id -u)}" -eq 0 ]`, d.Root()},
		{"in_docker_group?", `id -nG "${USER:-$(id -un)}" | grep -qw docker`, d.InDockerGroup()},
		{"add_to_docker_group", `sudo -n usermod -aG docker "${USER:-$(id -un)}"`, d.AddToDockerGroup()},
		{"refresh_session", "kill -HUP $PPID", d.RefreshSession()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { assertText(t, c.want, c.line.String()) })
	}
}

func TestServerEnsureRunDirectory(t *testing.T) {
	assertText(t, "mkdir -p .kamal", Server{newCase(t, simpleDeploy).base(t)}.EnsureRunDirectory().String())
}

const registryDeploy = `
service: app
image: dhh/app
registry: {username: dhh, password: secret, server: hub.docker.com}
builder: {arch: amd64}
servers: [1.1.1.1]
accessories:
  db:
    image: mysql:8.0
    hosts: [1.1.1.1]
    registry: {username: user, password: pw, server: other.hub.docker.com}
`

func registryLogin(t *testing.T, c *commandCase, accessory bool) string {
	t.Helper()
	cfg := c.config(t)
	registry := cfg.Registry()
	if accessory {
		registry = cfg.Accessory("db").Registry()
	}
	return must(t)(Registry{NewBase(cfg, c.runtime())}.Login(registry)).String()
}

func TestRegistryLogin(t *testing.T) {
	c := newCase(t, registryDeploy)
	t.Run("registry login", func(t *testing.T) {
		assertText(t, `docker login hub.docker.com -u "dhh" -p "secret"`, registryLogin(t, c, false))
	})
	t.Run("given registry login", func(t *testing.T) {
		assertText(t, `docker login other.hub.docker.com -u "user" -p "pw"`, registryLogin(t, c, true))
	})
}

func TestRegistryLoginRedactsCredentials(t *testing.T) {
	c := newCase(t, registryDeploy)
	cfg := c.config(t)
	line := must(t)(Registry{NewBase(cfg, c.runtime())}.Login(nil))
	assertText(t, "docker login hub.docker.com -u [REDACTED] -p [REDACTED]", line.Redacted())
}

func TestRegistryLoginWithEnvPassword(t *testing.T) {
	c := newCase(t, registryDeploy).secrets("KAMAL_REGISTRY_PASSWORD=more-secret\nKAMAL_MYSQL_REGISTRY_PASSWORD=secret-pw")
	c.set(t, `["KAMAL_REGISTRY_PASSWORD"]`, "registry", "password")
	c.set(t, `["KAMAL_MYSQL_REGISTRY_PASSWORD"]`, "accessories", "db", "registry", "password")
	assertText(t, `docker login hub.docker.com -u "dhh" -p "more-secret"`, registryLogin(t, c, false))
	assertText(t, `docker login other.hub.docker.com -u "user" -p "secret-pw"`, registryLogin(t, c, true))
}

func TestRegistryLoginEscapePassword(t *testing.T) {
	c := newCase(t, registryDeploy).secrets("KAMAL_REGISTRY_PASSWORD=more-secret'\"")
	c.set(t, `["KAMAL_REGISTRY_PASSWORD"]`, "registry", "password")
	assertText(t, `docker login hub.docker.com -u "dhh" -p "more-secret'\""`, registryLogin(t, c, false))
}

func TestRegistryLoginWithEnvUsername(t *testing.T) {
	c := newCase(t, registryDeploy).secrets("KAMAL_REGISTRY_USERNAME=also-secret")
	c.set(t, `["KAMAL_REGISTRY_USERNAME"]`, "registry", "username")
	assertText(t, `docker login hub.docker.com -u "also-secret" -p "secret"`, registryLogin(t, c, false))
}

func TestRegistryLogoutSetupAndRemove(t *testing.T) {
	c := newCase(t, registryDeploy)
	cfg := c.config(t)
	r := Registry{NewBase(cfg, c.runtime())}
	t.Run("registry logout", func(t *testing.T) { assertText(t, "docker logout hub.docker.com", r.Logout(nil).String()) })
	t.Run("given registry logout", func(t *testing.T) {
		assertText(t, "docker logout other.hub.docker.com", r.Logout(cfg.Accessory("db").Registry()).String())
	})
	t.Run("registry remove", func(t *testing.T) {
		assertText(t, "docker stop kamal-docker-registry && docker rm kamal-docker-registry", r.Remove().String())
	})
	t.Run("registry setup", func(t *testing.T) {
		local := newCase(t, registryDeploy)
		local.set(t, `{"server": "localhost:5000"}`, "registry")
		assertText(t, "docker start kamal-docker-registry || docker run --detach -p 127.0.0.1:5000:5000 --name kamal-docker-registry registry:3",
			Registry{local.base(t)}.Setup(nil).String())
	})
}

func pruneOf(t *testing.T, destination string) Prune {
	c := newCase(t, simpleDeploy)
	c.version, c.destination = "123", destination
	return Prune{c.base(t)}
}

func TestPrune(t *testing.T) {
	t.Run("dangling images", func(t *testing.T) {
		assertText(t, "docker image prune --force --filter label=service=app", pruneOf(t, "").DanglingImages().String())
	})
	tagged := `docker image ls --filter label=service=app --format '{{.ID}} {{.Repository}}:{{.Tag}}' | grep -v -w "$(docker container ls -a --format '{{.Image}}\|' --filter label=service=app | tr -d '\n')dhh/app:<none>\|[^ ]*:latest\(-[^ ]*\)\?$" | while read image tag; do docker rmi $tag; done`
	t.Run("tagged images", func(t *testing.T) { assertText(t, tagged, pruneOf(t, "").TaggedImages().String()) })
	t.Run("tagged images with destination", func(t *testing.T) { assertText(t, tagged, pruneOf(t, "staging").TaggedImages().String()) })
	containers := "docker ps -q -a --filter label=service=app --filter label=destination=%s --filter status=created --filter status=exited --filter status=dead | tail -n +%s | while read container_id; do docker rm $container_id; done"
	t.Run("app containers", func(t *testing.T) {
		assertText(t, sprintf(containers, "", "6"), pruneOf(t, "").AppContainers(5).String())
		assertText(t, sprintf(containers, "", "4"), pruneOf(t, "").AppContainers(3).String())
	})
	t.Run("app containers with destination", func(t *testing.T) {
		assertText(t, sprintf(containers, "staging", "6"), pruneOf(t, "staging").AppContainers(5).String())
	})
}

func sprintf(format string, args ...string) string {
	for _, arg := range args {
		format = strings.Replace(format, "%s", arg, 1)
	}
	return format
}

func lockOf(t *testing.T, userName string) Lock {
	c := newCase(t, simpleDeploy)
	c.version, c.destination = "123", "production"
	c.results["git config user.name"] = localshell.Result{Stdout: userName + "\n", Success: true}
	return Lock{c.base(t)}
}

func TestLock(t *testing.T) {
	l := lockOf(t, "Deployer")
	t.Run("status", func(t *testing.T) {
		assertText(t, "stat .kamal/lock-app-production > /dev/null && cat .kamal/lock-app-production/details | base64 -d", l.Status().String())
	})
	t.Run("acquire", func(t *testing.T) {
		assertMatch(t, `(?s)mkdir \.kamal/lock-app-production && echo ".*" > \.kamal/lock-app-production/details`, l.Acquire("Hello", "123").String())
	})
	t.Run("release", func(t *testing.T) {
		assertText(t, "rm .kamal/lock-app-production/details && rm -r .kamal/lock-app-production", l.Release().String())
	})
}

func TestAcquireEncodesNonASCIILockDetailsForShell(t *testing.T) {
	command := lockOf(t, "Сергей Федоров").Acquire("Hello", "123").String()
	encoded := regexp.MustCompile(`(?s)echo "(.*)" > \.kamal/lock-app-production/details`).FindStringSubmatch(command)[1]
	for _, r := range command {
		if r > 127 {
			t.Fatalf("command is not ASCII: %q", command)
		}
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(encoded, "\n", ""))
	if err != nil || !strings.Contains(string(decoded), "Locked by: Сергей Федоров") {
		t.Fatalf("decoded %q, %v", decoded, err)
	}
}

const auditPrefix = "[2026-11-11T10:30:00Z] [deployer@example.com]"

func auditorOf(t *testing.T, destination string, details ...tags.Tag) *Auditor {
	c := newCase(t, simpleDeploy)
	c.version, c.destination = "123", destination
	return NewAuditor(c.base(t), details...)
}

func TestAuditorRecord(t *testing.T) {
	record := func(extra, file string) []string {
		return []string{"mkdir", "-p", ".kamal", "&&", "echo", `"` + auditPrefix + extra + ` app removed container"`, ">>", file}
	}
	t.Run("record", func(t *testing.T) {
		assertWords(t, record("", ".kamal/app-audit.log"), auditorOf(t, "").Record("app removed container"))
	})
	t.Run("record with destination", func(t *testing.T) {
		assertWords(t, record(" [staging]", ".kamal/app-staging-audit.log"), auditorOf(t, "staging").Record("app removed container"))
	})
	t.Run("record with command details", func(t *testing.T) {
		assertWords(t, record(" [web]", ".kamal/app-audit.log"), auditorOf(t, "", tags.Tag{Name: "role", Value: "web"}).Record("app removed container"))
	})
	t.Run("record with arg details", func(t *testing.T) {
		assertWords(t, record(" [value]", ".kamal/app-audit.log"), auditorOf(t, "").Record("app removed container", tags.Tag{Name: "detail", Value: "value"}))
	})
}

func hookOf(t *testing.T, c *commandCase) Hook {
	c.version = "123"
	return Hook{c.base(t)}
}

func TestHookRun(t *testing.T) {
	assertWords(t, []string{".kamal/hooks/foo"}, hookOf(t, newCase(t, simpleDeploy)).Run("foo"))
}

func TestHookRunWithCustomHooksPath(t *testing.T) {
	custom := newCase(t, simpleDeploy).extra(t, "hooks_path", "custom/hooks/path")
	assertWords(t, []string{"custom/hooks/path/foo"}, hookOf(t, custom).Run("foo"))
}

const hookEnv = "KAMAL_RECORDED_AT=2026-11-11T10:30:00Z KAMAL_PERFORMER=deployer@example.com KAMAL_VERSION=123 KAMAL_SERVICE_VERSION=app@123 KAMAL_SERVICE=app"

func TestHookEnv(t *testing.T) {
	env, err := hookOf(t, newCase(t, simpleDeploy)).Env(false)
	assertText(t, hookEnv, tagPairs(t, env, err))
}

func TestHookEnvWithSecrets(t *testing.T) {
	env, err := hookOf(t, newCase(t, simpleDeploy).secrets("DB_PASSWORD=secret")).Env(true)
	assertText(t, hookEnv+" DB_PASSWORD=secret", tagPairs(t, env, err))
}

func tagPairs(t *testing.T, env []tags.Tag, err error) string {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	var pairs []string
	for _, tag := range env {
		pairs = append(pairs, tag.Name+"="+tag.Value)
	}
	return strings.Join(pairs, " ")
}
