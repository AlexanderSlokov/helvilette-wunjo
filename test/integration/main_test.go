//go:build integration

package integration

// Ports test/integration/main_test.rb at Kamal 2.12.0. The Ruby test reads
// Kamal::VERSION where the port reads the Wunjo version.

import (
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/cli"
)

func assertMatches(t *testing.T, text string, patterns ...string) {
	t.Helper()
	for _, pattern := range patterns {
		if !regexp.MustCompile(pattern).MatchString(text) {
			t.Fatalf("output lacks /%s/:\n%s", pattern, text)
		}
	}
}

func assertNoMatches(t *testing.T, text string, patterns ...string) {
	t.Helper()
	for _, pattern := range patterns {
		if regexp.MustCompile(pattern).MatchString(text) {
			t.Fatalf("output has /%s/:\n%s", pattern, text)
		}
	}
}

func TestDeployRedeployRollbackDetailsAndAudit(t *testing.T) {
	s := newStack(t)
	first := s.latestAppVersion()
	s.assertAppIsDown()

	deployOutput := s.wunjo("deploy")
	s.assertAppIsUp(first)
	s.assertHooksRan("pre-connect", "pre-build", "pre-deploy", "pre-app-boot", "post-app-boot", "post-deploy")
	assertHookOutput(t, deployOutput)
	assertMatches(t, deployOutput, `Logs written to /tmp/kamal-deploy-logs/.*_deploy\.log`, `Logs sent to http://otel_collector:4318`)
	s.assertDeployLog("*_deploy.log", `Build and push app image`, `INFO .* Running docker`, `post-deploy`)
	s.assertEnvs(first)
	s.assertHookEnvVariables(s.wunjo("app", "exec", "--verbose", "ls", "-r", "web"), first)

	second := s.updateAppRev()
	s.wunjo("redeploy")
	s.assertAppIsUp(second)
	s.assertHooksRan("pre-connect", "pre-build", "pre-deploy", "pre-app-boot", "post-app-boot", "post-deploy")
	s.assertDeployLog("*_redeploy.log", `Build and push app image`, `INFO .* Running docker`)
	s.assertAccumulatedAssets(first, second)
	s.assertAssetVolumeReadOnly(second)

	s.wunjo("rollback", first)
	s.assertHooksRan("pre-connect", "pre-deploy", "pre-app-boot", "post-app-boot", "post-deploy")
	s.assertAppIsUp(first)
	s.assertDeployLog("*_rollback.log", `INFO .* Running docker`, `pre-connect`)

	assertMatches(t, s.wunjo("details"), `Proxy Host: vm1`, `Proxy Host: vm2`, `App Host: vm1`, `App Host: vm2`,
		`basecamp/kamal-proxy:v0\.9\.2`, `localhost:5000/app:`+first)
	assertMatches(t, s.wunjo("audit"), `(?s)Booted app version `+first+`.*Booted app version `+second+`.*Booted app version `+first)
	s.assertOtelLogs()
}

func TestAppWithRoles(t *testing.T) {
	s := newStack(t)
	s.app = "app_with_roles"
	version := s.latestAppVersion()
	s.assertAppIsDown()
	s.wunjo("deploy")
	s.assertAppIsUp(version)
	s.assertHooksRan("pre-connect", "pre-build", "pre-deploy", "post-deploy")
	s.assertContainerRunning("vm3", "app_with_roles-workers-"+version)

	second := s.updateAppRev()
	s.wunjo("redeploy")
	s.assertAppIsUp(second)
	s.assertContainerRunning("vm3", "app_with_roles-workers-"+second)
}

func localArch() string {
	if runtime.GOARCH == "arm64" {
		return "arm64"
	}
	return "amd64"
}

func TestConfig(t *testing.T) {
	s := newStack(t)
	config, version := s.wunjo("config"), s.latestAppVersion()
	for _, want := range []string{
		":roles:\n- web\n", ":hosts:\n- vm1\n- vm2\n- vm3\n", ":primary_host: vm1\n", ":version: " + version + "\n",
		":repository: localhost:5000/app\n", ":absolute_image: localhost:5000/app:" + version + "\n",
		":service_with_version: app-" + version + "\n", ":volume_args: []\n",
		":ssh_options:\n  :user: root\n  :port: 22\n  :keepalive: true\n  :keepalive_interval: 30\n  :log_level: :fatal\n",
		":builder:\n  driver: docker\n  arch: " + localArch() + "\n  args:\n    COMMIT_SHA: " + version + "\n",
		":logging:\n- \"--log-opt\"\n- max-size=\"10m\"",
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("config lacks %q:\n%s", want, config)
		}
	}
}

func TestAliases(t *testing.T) {
	s := newStack(t)
	s.app = "app_with_roles"
	s.wunjo("deploy")
	if got := s.wunjo("whome"); got != cli.Version {
		t.Fatalf("whome = %q, want %q", got, cli.Version)
	}
	assertMatches(t, s.wunjo("worker_hostname"), `App Host: vm3\nvm3-[0-9a-f]{12}$`)
	assertMatches(t, s.wunjo("worker_hostname_quiet"), `vm3-[0-9a-f]{12}$`)
	assertMatches(t, s.wunjo("uname", "-o"), `App Host: vm1\nGNU/Linux`)
	assertMatches(t, s.wunjo("uname_quiet", "-o"), `GNU/Linux`)
}

func TestDeployAndPruneWithDestinations(t *testing.T) {
	s := newStack(t)
	s.app = "app_with_destinations"
	s.wunjo("staging_deploy")
	s.assertAppIsUp("")
	assertMatches(t, s.wunjo("staging_config"), `:hosts:\n- vm1\n:`)
	assertMatches(t, s.wunjo("production_config"), `:hosts:\n- vm2\n- vm3\n:`)

	image := "registry:4443/app_with_destinations:" + s.latestAppVersion()
	for _, c := range [][2]string{{"production-old", "production"}, {"staging-old", "staging"}, {"staging-new", "staging"}} {
		s.onHost("vm1", "docker create --name "+c[0]+" --label service=app_with_destinations --label destination="+c[1]+" "+image)
	}
	s.wunjo("prune", "containers", "-d", "staging", "--retain", "1")
	containers := strings.Fields(s.onHost("vm1", "docker container ls -a --format '{{.Names}}'"))
	assertIncludes(t, containers, []string{"production-old", "staging-new"}, []string{"staging-old"})

	for _, tag := range []string{"latest", "latest-production", "latest.backup", "unused"} {
		s.onHost("vm1", "docker tag "+image+" other/app:"+tag)
	}
	s.wunjo("prune", "images", "-d", "staging")
	images := strings.Fields(s.onHost("vm1", "docker image ls --filter label=service=app_with_destinations --format '{{.Repository}}:{{.Tag}}'"))
	assertIncludes(t, images, []string{"registry:4443/app_with_destinations:latest-staging", "other/app:latest", "other/app:latest-production"},
		[]string{"other/app:latest.backup", "other/app:unused"})
}

func assertIncludes(t *testing.T, items, present, absent []string) {
	t.Helper()
	joined := "\n" + strings.Join(items, "\n") + "\n"
	for _, item := range present {
		if !strings.Contains(joined, "\n"+item+"\n") {
			t.Fatalf("%v lacks %s", items, item)
		}
	}
	for _, item := range absent {
		if strings.Contains(joined, "\n"+item+"\n") {
			t.Fatalf("%v has %s", items, item)
		}
	}
}

func TestSetupAndRemove(t *testing.T) {
	s := newStack(t)
	s.wunjo("proxy", "boot_config", "set", "--publish=false",
		"--docker-options=label=traefik.http.services.kamal_proxy.loadbalancer.server.scheme=http",
		"label=traefik.http.routers.kamal_proxy.rule=PathPrefix(`/`)",
		"label=traefik.http.routers.kamal_proxy.priority=2")

	// remove completes when nothing is set up yet
	s.wunjo("remove", "-y")
	s.assertImagesAndContainers(false)

	s.wunjo("setup")
	s.assertImagesAndContainers(true)

	s.wunjo("remove", "-y")
	s.assertImagesAndContainers(false)
	s.assertAppDirectoryRemoved()
}

// assertImagesAndContainers checks that each vm has images and containers, or none.
func (s *stack) assertImagesAndContainers(want bool) {
	s.t.Helper()
	for _, vm := range []string{"vm1", "vm2", "vm3"} {
		images, containers := s.onHost(vm, "docker image ls -q"), s.onHost(vm, "docker ps -a -q")
		if (images != "") != want || (containers != "") != want {
			s.t.Fatalf("%s: images %q, containers %q, want present = %v", vm, images, containers, want)
		}
	}
}

func TestTwoApps(t *testing.T) {
	s := newStack(t)
	s.wunjo("deploy")
	appVersion := s.latestAppVersion()
	s.app = "app_with_roles"
	s.wunjo("deploy")
	rolesVersion := s.latestAppVersion()

	s.assertAppIsUpAs("app", appVersion, "")
	s.assertAppIsUpAs("app_with_roles", rolesVersion, "")

	s.app = "app"
	s.wunjo("remove", "-y")
	s.assertAppDirectoryRemoved()
	s.assertProxyRunning()

	s.app = "app_with_roles"
	s.wunjo("remove", "-y")
	s.assertAppDirectoryRemoved()
	s.assertProxyNotRunning()
}

func TestDeployWithTraefik(t *testing.T) {
	s := newStack(t)
	s.app = "app_with_traefik"
	version := s.latestAppVersion()
	s.wunjo("setup")
	s.assertAppIsUp(version)
}

func TestDeployWithACustomCertificate(t *testing.T) {
	s := newStack(t)
	s.app = "app_with_custom_certificate"
	version := s.latestAppVersion()
	s.wunjo("setup")
	s.assertAppIsUpAs(s.app, version, "docker/deployer/app_with_custom_certificate/certs/cert.pem")
}
