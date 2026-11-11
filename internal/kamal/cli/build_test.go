package cli

// Ports test/cli/build_test.rb at Kamal 2.12.0. The fake Git checkout lives at /k,
// so the clone directory is /tmp/kamal-clones/app-<sha of /k>.

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
)

const revision = "0123456789abcdef0123456789abcdef01234567"

func pwdSHA() string {
	sum := sha256.Sum256([]byte("/k"))
	return hex.EncodeToString(sum[:])[:13]
}

// buildWorld is a Git checkout at /k with a clean clone at the expected revision.
func buildWorld(t *testing.T, fixture string) *world {
	w := newWorld(t).fixture("deploy_" + fixture)
	w.shell["git rev-parse"] = localshell.Result{Success: true}
	w.shell["git rev-parse HEAD"] = localshell.Result{Stdout: revision + "\n", Success: true}
	w.shell["git status --porcelain"] = localshell.Result{Success: true}
	w.shell["git rev-parse --show-toplevel"] = localshell.Result{Stdout: "/k\n", Success: true}
	w.stubLocal(contains("rev-parse HEAD"), revision, "").stubLocal(contains("status --porcelain"), "", "")
	return w
}

func buildRun(w *world, fixture string, args ...string) string {
	w.t.Helper()
	return w.run(append(append([]string{"build"}, args...), "-c", "test/fixtures/deploy_"+fixture+".yml")...)
}

func TestBuildDeliver(t *testing.T) {
	output := buildRun(buildWorld(t, "with_accessories"), "with_accessories", "deliver")
	assertContains(t, output, "docker buildx build --output=type=registry")
	assertContains(t, output, "docker pull dhh/app:999")
}

func TestBuildPush(t *testing.T) {
	for output, wantOutput := range map[string]string{"": "registry", "--output=docker": "docker"} {
		args := []string{"push", "--verbose"}
		if output != "" {
			args = append(args, output)
		}
		result := buildRun(buildWorld(t, "with_accessories").hooksExist(), "with_accessories", args...)
		assertHookRan(t, result, "pre-connect", 1)
		assertHookRan(t, result, "pre-build", 1)
		assertContains(t, result, "Cloning repo into build directory")
		assertContains(t, result, "git -C /tmp/kamal-clones/app-"+pwdSHA()+" clone /k")
		assertContains(t, result, "docker --version && docker buildx version")
		assertMatches(t, result, `docker buildx build --output=type=`+wantOutput+` --platform linux/amd64 --builder kamal-local-docker-container-6e208c509c5d -t dhh/app:999 -t dhh/app:latest --label service="app" --file Dockerfile \. 2>&1 as .*@localhost`)
	}
}

func TestBuildPushWithRemoteBuilder(t *testing.T) {
	output := buildRun(buildWorld(t, "with_remote_builder").hooksExist(), "with_remote_builder", "push", "--verbose")
	assertNotContains(t, output, "Running docker login -u [REDACTED] -p [REDACTED] as ")
	assertContains(t, output, "docker buildx inspect kamal-remote-ssh---app-1-1-1-5-6e208c509c5d | grep -q Endpoint:.*kamal-remote-ssh---app-1-1-1-5-6e208c509c5d-context && docker context inspect kamal-remote-ssh---app-1-1-1-5-6e208c509c5d")
	assertContains(t, output, `Command: ( export BUILDKIT_NO_CLIENT_TOKEN="1" ; docker buildx build --output=type=registry --platform linux/arm64 --builder kamal-remote-ssh---app-1-1-1-5-6e208c509c5d -t dhh/app:999 -t dhh/app:latest`)
}

func TestBuildPushWithLocalRegistryStartsTheRegistryFirst(t *testing.T) {
	for _, fixture := range []string{"with_local_registry_and_remote_builder", "with_local_registry_and_hybrid_builder"} {
		output := buildRun(buildWorld(t, fixture).hooksExist(), fixture, "push", "--verbose")
		setup := "docker start kamal-docker-registry || docker run --detach -p 127.0.0.1:5000:5000 --name kamal-docker-registry registry:3"
		assertContains(t, output, setup)
		if strings.Index(output, setup) > strings.Index(output, "docker buildx build") {
			t.Fatalf("%s: registry starts after the build", fixture)
		}
		assertNotContains(t, output, "Running docker login -u [REDACTED] -p [REDACTED] as ")
		assertContains(t, output, `BUILDKIT_NO_CLIENT_TOKEN="1"`)
	}
}

func TestBuildPushResettingClone(t *testing.T) {
	w := buildWorld(t, "with_accessories")
	w.local.stubs = append([]stub{failTimes(contains(" clone /k --recurse-submodules"), 1, "fatal: destination path 'kamal' already exists and is not an empty directory")}, w.local.stubs...)
	output := buildRun(w, "with_accessories", "push", "--verbose")
	assertContains(t, output, "Cloning repo into build directory")
	assertContains(t, output, "Resetting local clone")
}

func TestBuildPushWithCorruptClone(t *testing.T) {
	w := buildWorld(t, "with_accessories")
	w.local.stubs = append([]stub{
		failTimes(contains(" clone /k --recurse-submodules"), 1, "fatal: destination path 'kamal' already exists and is not an empty directory"),
		{match: contains("remote set-url origin"), fail: "fatal: not a git repository"},
	}, w.local.stubs...)
	output := buildRun(w, "with_accessories", "push", "--verbose")
	dir := regexp.QuoteMeta("/tmp/kamal-clones/app-" + pwdSHA() + "/k/")
	assertMatches(t, output, "(?s)Cloning repo into build directory `"+dir+"`\\.\\.\\..*Cloning repo into build directory `"+dir+"`")
	assertContains(t, output, "Resetting local clone as `/tmp/kamal-clones/app-"+pwdSHA()+"/k/` already exists...")
	assertContains(t, output, "Error preparing clone: Failed to clone repo: fatal: not a git repository, deleting and retrying...")
}

func TestBuildPushWithoutClone(t *testing.T) {
	for _, extra := range [][]string{nil, {"--no-cache"}} {
		output := buildRun(buildWorld(t, "without_clone").hooksExist(), "without_clone", append([]string{"push", "--verbose"}, extra...)...)
		assertNotContains(t, output, "Cloning repo into build directory")
		assertHookRan(t, output, "pre-build", 1)
		want := `--label service="app" --file Dockerfile . 2>&1`
		if extra != nil {
			want = `--label service="app" --file Dockerfile --no-cache . 2>&1`
		}
		assertContains(t, output, "docker buildx build --output=type=registry --platform linux/amd64 --builder kamal-local-docker-container-6e208c509c5d -t dhh/app:999 -t dhh/app:latest "+want)
	}
}

func TestBuildPushWithoutBuilder(t *testing.T) {
	cases := map[string]string{"with_accessories": "kamal-local-docker-container-6e208c509c5d", "with_local_registry_and_accessories": "kamal-local-registry-docker-container-3e4092d0c8b9"}
	for fixture, builder := range cases {
		w := buildWorld(t, fixture).stubLocal(equals("docker buildx inspect "+builder), "", "no builder")
		output := buildRun(w, fixture, "push")
		assertMatches(t, output, `WARN Missing compatible builder, so creating a new one first`)
		assertContains(t, output, "docker buildx rm "+builder)
		assertContains(t, output, "docker buildx create --name "+builder+" --driver=docker-container")
	}
}

func TestBuildPushWithNoBuildxPlugin(t *testing.T) {
	w := buildWorld(t, "with_accessories").stubLocal(equals("docker --version && docker buildx version"), "", "no buildx")
	assertContains(t, w.runFailing("build", "push", "-c", accessoriesConfig), "ERROR (Kamal::Cli::DependencyError)")
}

func TestBuildPushPreBuildHookFailure(t *testing.T) {
	w := buildWorld(t, "with_accessories").hooksExist().stubLocal(equals("/usr/bin/env .kamal/hooks/pre-build"), "", "failed")
	output := w.runFailing("build", "push", "-c", accessoriesConfig)
	assertContains(t, output, "ERROR (Kamal::Cli::HookError): Hook `pre-build` failed:\nfailed")
	assertNotContains(t, output, "docker buildx build")
}

const validateImage = `docker inspect -f '{{ .Config.Labels.service }}' dhh/app:999 | grep -x app || (echo "Image dhh/app:999 is missing the 'service' label" && exit 1)`

func TestBuildPull(t *testing.T) {
	output := buildRun(buildWorld(t, "with_accessories"), "with_accessories", "pull")
	for _, want := range []string{"docker info --format '{{index .RegistryConfig.Mirrors 0}}'", "docker image rm --force dhh/app:999", "docker pull dhh/app:999", validateImage} {
		assertContains(t, output, want)
	}
}

func TestBuildPullWithMirrors(t *testing.T) {
	const firstMirror = "docker info --format '{{index .RegistryConfig.Mirrors 0}}'"
	cases := map[string][]string{
		`Pulling image on 1\.1\.1\.\d to seed the mirror\.\.\.`:               {"registry-mirror.example.com"},
		`Pulling image on 1\.1\.1\.\d, 1\.1\.1\.\d to seed the mirrors\.\.\.`: {"registry-mirror.example.com", "registry-mirror2.example.com"},
	}
	for want, mirrors := range cases {
		w := buildWorld(t, "with_accessories")
		w.hosts.stubs = append(w.hosts.stubs, sequenceStub(equals(firstMirror), mirrors...))
		output := buildRun(w, "with_accessories", "pull")
		assertMatches(t, output, want)
		assertContains(t, output, "Pulling image on remaining hosts...")
		if n := strings.Count(output, "docker pull dhh/app:999"); n != 4 {
			t.Fatalf("%d pulls, want 4:\n%s", n, output)
		}
	}
}

func TestBuildCreate(t *testing.T) {
	cases := map[string][]string{
		"with_accessories": {"docker buildx create --name kamal-local-docker-container-6e208c509c5d --driver=docker-container"},
		"with_remote_builder": {"Running /usr/bin/env true on 1.1.1.5",
			"docker context create kamal-remote-ssh---app-1-1-1-5-6e208c509c5d-context --description 'kamal-remote-ssh---app-1-1-1-5-6e208c509c5d host' --docker 'host=ssh://app@1.1.1.5'",
			"docker buildx create --name kamal-remote-ssh---app-1-1-1-5-6e208c509c5d kamal-remote-ssh---app-1-1-1-5-6e208c509c5d-context"},
		"with_remote_builder_and_custom_ports": {"Running /usr/bin/env true on 1.1.1.5",
			"docker context create kamal-remote-ssh---app-1-1-1-5-2122-6e208c509c5d-context --description 'kamal-remote-ssh---app-1-1-1-5-2122-6e208c509c5d host' --docker 'host=ssh://app@1.1.1.5:2122'"},
		"with_hybrid_builder": {"Running /usr/bin/env true on 1.1.1.5",
			"docker buildx create --platform linux/amd64 --name kamal-hybrid-docker-container-ssh---app-1-1-1-5-6e208c509c5d --driver=docker-container",
			"docker buildx create --platform linux/arm64 --append --name kamal-hybrid-docker-container-ssh---app-1-1-1-5-6e208c509c5d kamal-hybrid-docker-container-ssh---app-1-1-1-5-6e208c509c5d-context"},
		"with_cloud_builder":                               {"docker buildx create --driver cloud example_org/cloud_builder"},
		"with_local_registry":                              {"docker buildx create --name kamal-local-registry-docker-container-3e4092d0c8b9 --driver=docker-container --driver-opt network=host"},
		"with_local_registry_and_remote_builder":           {"docker buildx create --name kamal-remote-ssh---app-1-1-1-5-local-registry-3e4092d0c8b9", "--driver-opt network=host"},
		"with_local_registry_and_remote_builder_with_port": {"docker buildx create --name kamal-remote-ssh---app-1-1-1-5-2222-local-registry-3e4092d0c8b9", "--driver-opt network=host"},
	}
	for fixture, wants := range cases {
		output := buildRun(buildWorld(t, fixture), fixture, "create")
		for _, want := range wants {
			assertContains(t, output, want)
		}
	}
}

func TestBuildCreateWithError(t *testing.T) {
	w := buildWorld(t, "with_accessories").stubLocal(contains("docker buildx create"), "", "stderr=error")
	assertContains(t, buildRun(w, "with_accessories", "create"), "Couldn't create remote builder: error")
}

func TestBuildRemoveAndDetails(t *testing.T) {
	assertContains(t, buildRun(buildWorld(t, "with_accessories"), "with_accessories", "remove"), "docker buildx rm kamal-local")
	assertContains(t, buildRun(buildWorld(t, "with_cloud_builder"), "with_cloud_builder", "remove"), "docker buildx rm cloud-example_org-cloud_builder")
	w := buildWorld(t, "with_accessories").stubLocal(equals("docker context ls && docker buildx ls"), "docker builder info", "")
	output := buildRun(w, "with_accessories", "details")
	assertContains(t, output, "Builder: local")
	assertContains(t, output, "docker builder info")
}

func TestBuildDev(t *testing.T) {
	cases := map[string][]string{
		`--output=type=docker --platform linux/amd64 --builder kamal-local-docker-container-6e208c509c5d -t dhh/app:999-dirty -t dhh/app:latest-dirty --label service="app" --file Dockerfile . 2>&1`:            nil,
		`--output=type=local --platform linux/amd64 --builder kamal-local-docker-container-6e208c509c5d -t dhh/app:999-dirty -t dhh/app:latest-dirty --label service="app" --file Dockerfile . 2>&1`:             {"--output=local"},
		`--output=type=docker --platform linux/amd64 --builder kamal-local-docker-container-6e208c509c5d -t dhh/app:999-dirty -t dhh/app:latest-dirty --label service="app" --file Dockerfile --no-cache . 2>&1`: {"--no-cache"},
	}
	for want, extra := range cases {
		output := buildRun(buildWorld(t, "with_accessories").hooksExist(), "with_accessories", append([]string{"dev", "--verbose"}, extra...)...)
		assertNotContains(t, output, "Cloning repo into build directory")
		assertContains(t, output, "docker --version && docker buildx version")
		assertContains(t, output, "docker buildx build "+want)
	}
}

func TestBuildPullWithLocalRegistry(t *testing.T) {
	output := buildRun(buildWorld(t, "with_local_registry"), "with_local_registry", "pull")
	assertContains(t, output, "docker pull localhost:5000/dhh/app:999")
}
