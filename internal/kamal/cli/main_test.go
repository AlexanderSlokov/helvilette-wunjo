package cli

// Ports the setup, deploy, lock, redeploy, remove, details, and upgrade tests of
// test/cli/main_test.rb at Kamal 2.12.0. A test that expects invoke in Ruby sets
// recordInvokes, so the invoked commands record their options and run nothing.

import (
	"slices"
	"strings"
	"testing"
)

const lockExists = "mkdir: cannot create directory ‘kamal/lock-app’: File exists"

// mainWorld loads test/fixtures/<fixture>.yml and records invocations.
func mainWorld(t *testing.T, fixture string) *world {
	w := newWorld(t).fixture(fixture)
	w.recordInvokes = true
	return w
}

func mainRun(w *world, fixture string, args ...string) string {
	w.t.Helper()
	return w.run(append(args, "-c", "test/fixtures/"+fixture+".yml")...)
}

// invokeOptions returns the options of base_invoke_options for the config file.
func invokeOptions(configFile string, extras Options) Options {
	return Options{"config_file": "test/fixtures/" + configFile, "skip_hooks": "false", "lock_wait": "false",
		"lock_wait_timeout": "900", "lock_wait_interval": "15", "version": "999"}.Merge(extras)
}

// deployInvocations are the commands that deploy invokes after build:deliver or
// build:pull.
func deployInvocations(build string, opts Options) []string {
	return []string{invocation(build, nil, opts), invocation("proxy:boot", nil, opts),
		invocation("app:stale_containers", nil, opts.With("stop", "true")), invocation("app:boot", nil, opts),
		invocation("prune:all", nil, opts)}
}

func assertInvoked(t *testing.T, w *world, want ...string) {
	t.Helper()
	for _, call := range want {
		if !slices.Contains(w.invokes, call) {
			t.Fatalf("no invocation %q in:\n%s", call, strings.Join(w.invokes, "\n"))
		}
	}
}

func withTestSecrets(w *world) *world {
	w.files[".kamal/secrets"] = "DB_PASSWORD=secret"
	return w
}

func TestSetup(t *testing.T) {
	w := mainWorld(t, "deploy_simple")
	assertContains(t, mainRun(w, "deploy_simple", "setup"), "Ensure Docker is installed...")
	assertInvoked(t, w, invocation("server:bootstrap", nil, invokeOptions("deploy_simple.yml", nil)))
}

func TestSetupWithSkipPush(t *testing.T) {
	w := mainWorld(t, "deploy_simple")
	output := mainRun(w, "deploy_simple", "setup", "--skip_push")
	opts := invokeOptions("deploy_simple.yml", nil)
	assertInvoked(t, w, append(deployInvocations("build:pull", opts),
		invocation("server:bootstrap", nil, opts), invocation("accessory:boot", []string{"all"}, opts))...)
	for _, want := range []string{"Ensure Docker is installed...", "Acquiring the deploy lock", "Pull app image",
		"Ensure kamal-proxy is running", "Detect stale containers", "Prune old containers and images", "Releasing the deploy lock"} {
		assertContains(t, output, want)
	}
}

func TestSetupWithNoCache(t *testing.T) {
	w := mainWorld(t, "deploy_simple")
	output := mainRun(w, "deploy_simple", "setup", "--no-cache")
	opts := invokeOptions("deploy_simple.yml", Options{"no_cache": "true"})
	assertInvoked(t, w, append(deployInvocations("build:deliver", opts),
		invocation("server:bootstrap", nil, opts), invocation("accessory:boot", []string{"all"}, opts))...)
	assertDeploySteps(t, output)
}

func assertDeploySteps(t *testing.T, output string) {
	t.Helper()
	for _, want := range []string{"Build and push app image", "Ensure kamal-proxy is running", "Detect stale containers", "Prune old containers and images"} {
		assertContains(t, output, want)
	}
}

func TestDeploy(t *testing.T) {
	for _, fixture := range []string{"deploy_simple", "deploy_with_local_registry"} {
		w := withTestSecrets(mainWorld(t, fixture)).hooksExist()
		output := mainRun(w, fixture, "deploy", "--verbose")
		assertInvoked(t, w, deployInvocations("build:deliver", invokeOptions(fixture+".yml", Options{"verbose": "true"}))...)
		assertDeploySteps(t, output)
		for _, hook := range []string{"pre-connect", "pre-deploy", "post-deploy"} {
			assertHookRan(t, output, hook, 1)
		}
	}
}

func TestDeployWithSkipPush(t *testing.T) {
	w := mainWorld(t, "deploy_simple")
	output := mainRun(w, "deploy_simple", "deploy", "--skip_push")
	assertInvoked(t, w, deployInvocations("build:pull", invokeOptions("deploy_simple.yml", nil))...)
	for _, want := range []string{"Acquiring the deploy lock", "Pull app image", "Ensure kamal-proxy is running", "Releasing the deploy lock"} {
		assertContains(t, output, want)
	}
}

func TestDeployWithNoCache(t *testing.T) {
	w := mainWorld(t, "deploy_simple")
	assertDeploySteps(t, mainRun(w, "deploy_simple", "deploy", "--no-cache"))
	assertInvoked(t, w, deployInvocations("build:deliver", invokeOptions("deploy_simple.yml", Options{"no_cache": "true"}))...)
}

// lockedWorld builds for real and fails to create the lock directory, as the
// Ruby lock tests stub mkdir to raise.
func lockedWorld(t *testing.T, failure string) *world {
	return buildWorld(t, "simple").stubRemote(contains("mkdir .kamal/lock-app"), "", failure)
}

func TestDeployWhenLocked(t *testing.T) {
	w := lockedWorld(t, lockExists)
	assertContains(t, w.runFailing("deploy", "-c", "test/fixtures/deploy_simple.yml"), "ERROR (Kamal::Cli::LockError)")
}

func TestDeployWithLockWaitRetriesAndAcquiresLockWhenFreed(t *testing.T) {
	w := mainWorld(t, "deploy_simple")
	w.hosts.stubs = append(w.hosts.stubs, failTimes(contains("mkdir .kamal/lock-app"), 2, lockExists))
	w.stubCapture(contains("base64 -d"), "Locked by: alice\nVersion: 999\nMessage: Automatic deploy lock")
	output := mainRun(w, "deploy_simple", "deploy", "--lock-wait", "--lock-wait-interval", "0", "--lock-wait-timeout", "60")
	for _, want := range []string{"Acquiring the deploy lock (waiting up to 60s)", "Deploy lock is held by:", "Retrying in 0s", "Releasing the deploy lock"} {
		assertContains(t, output, want)
	}
}

func TestDeployWithLockWaitTimesOut(t *testing.T) {
	w := lockedWorld(t, lockExists).stubCapture(contains("base64 -d"), "Locked by: alice\nVersion: 999\nMessage: Automatic deploy lock")
	output := w.runFailing("deploy", "--lock-wait", "--lock-wait-timeout", "0", "--lock-wait-interval", "0", "-c", "test/fixtures/deploy_simple.yml")
	assertContains(t, output, "ERROR (Kamal::Cli::LockError)")
}

func TestDeployWithLockWaitFailsImmediatelyWhenTheLockIsHeldManually(t *testing.T) {
	w := lockedWorld(t, lockExists).stubCapture(contains("base64 -d"), "Locked by: alice\nVersion: 999\nMessage: Stopping deploys for maintenance")
	output := w.runFailing("deploy", "--lock-wait", "--lock-wait-timeout", "60", "--lock-wait-interval", "0", "-c", "test/fixtures/deploy_simple.yml")
	assertMatches(t, output, `ERROR \(Kamal::Cli::LockError\): .*held manually`)
	if len(w.sleeps) != 0 {
		t.Fatalf("slept %v, want no wait for a manual lock", w.sleeps)
	}
}

func TestDeployWhenInheritingLock(t *testing.T) {
	w := mainWorld(t, "deploy_simple").hooksExist()
	w.holdingLock = true
	output := mainRun(w, "deploy_simple", "deploy")
	assertInvoked(t, w, deployInvocations("build:deliver", invokeOptions("deploy_simple.yml", nil))...)
	assertDeploySteps(t, output)
	assertNotContains(t, output, "Acquiring the deploy lock")
	assertNotContains(t, output, "Releasing the deploy lock")
}

func TestDeployErrorWhenLocking(t *testing.T) {
	w := lockedWorld(t, "getaddrinfo: nodename nor servname provided, or not known")
	// bin/kamal names the cause of an SSHKit::Runner::ExecuteError. The cause is
	// SocketError in Ruby and the stub's failure here.
	assertContains(t, w.runFailing("deploy", "-c", "test/fixtures/deploy_simple.yml"),
		"ERROR (SSHKit::Command::Failed): Exception while executing on host 1.1.1.1: getaddrinfo: nodename nor servname provided, or not known")
}

func TestDeployErrorsDuringOutsideSectionLeaveRemoteLock(t *testing.T) {
	w := mainWorld(t, "deploy_simple")
	w.failInvoke = "build:deliver"
	output := w.runFailing("deploy", "-c", "test/fixtures/deploy_simple.yml")
	if w.last.kamal.HoldingLock {
		t.Fatal("holding the lock after a failed build, want no lock")
	}
	assertNotContains(t, output, "Acquiring the deploy lock")
}

func TestDeployWithSkippedHooks(t *testing.T) {
	w := mainWorld(t, "deploy_simple").hooksExist()
	output := mainRun(w, "deploy_simple", "deploy", "--skip_hooks")
	assertInvoked(t, w, deployInvocations("build:deliver", invokeOptions("deploy_simple.yml", Options{"skip_hooks": "true"}))...)
	assertNotContains(t, output, ".kamal/hooks/post-deploy")
}

func TestDeployWithMissingSecrets(t *testing.T) {
	w := mainWorld(t, "deploy_with_secrets")
	mainRun(w, "deploy_with_secrets", "deploy")
	assertInvoked(t, w, deployInvocations("build:deliver", invokeOptions("deploy_with_secrets.yml", nil))...)
}

// redeployInvocations are the commands that redeploy invokes.
func redeployInvocations(build string, opts Options) []string {
	return []string{invocation(build, nil, opts), invocation("app:stale_containers", nil, opts.With("stop", "true")),
		invocation("app:boot", nil, opts)}
}

func TestRedeploy(t *testing.T) {
	w := mainWorld(t, "deploy_simple").hooksExist()
	output := mainRun(w, "deploy_simple", "redeploy", "--verbose")
	assertInvoked(t, w, redeployInvocations("build:deliver", invokeOptions("deploy_simple.yml", Options{"verbose": "true"}))...)
	assertContains(t, output, "Build and push app image")
	assertContains(t, output, "Running /usr/bin/env .kamal/hooks/pre-deploy ")
	for _, hook := range []string{"pre-connect", "pre-deploy", "post-deploy"} {
		assertHookRan(t, output, hook, 1)
	}
}

func TestRedeployWithSkipPushAndNoCache(t *testing.T) {
	w := mainWorld(t, "deploy_simple")
	assertContains(t, mainRun(w, "deploy_simple", "redeploy", "--skip_push"), "Pull app image")
	assertInvoked(t, w, redeployInvocations("build:pull", invokeOptions("deploy_simple.yml", nil))...)
	w = mainWorld(t, "deploy_simple")
	assertContains(t, mainRun(w, "deploy_simple", "redeploy", "--no-cache"), "Build and push app image")
	assertInvoked(t, w, redeployInvocations("build:deliver", invokeOptions("deploy_simple.yml", Options{"no_cache": "true"}))...)
}

func TestRemove(t *testing.T) {
	w := mainWorld(t, "deploy_simple")
	mainRun(w, "deploy_simple", "remove", "-y")
	opts := invokeOptions("deploy_simple.yml", Options{"confirmed": "true"}).Without("version")
	assertInvoked(t, w, invocation("proxy:remove", nil, opts), invocation("app:remove", nil, opts),
		invocation("accessory:remove", []string{"all"}, opts), invocation("registry:remove", nil, opts.With("skip_local", "true")))
}

func TestDetails(t *testing.T) {
	w := mainWorld(t, "deploy_simple")
	mainRun(w, "deploy_simple", "details")
	assertInvokedNames(t, w, "proxy:details", "app:details", "accessory:details all")
}

// assertInvokedNames checks the commands and arguments of the invocations, as a
// Ruby expectation without options does.
func assertInvokedNames(t *testing.T, w *world, want ...string) {
	t.Helper()
	var names []string
	for _, call := range w.invokes {
		names = append(names, strings.SplitN(call, " | ", 2)[0])
	}
	if !slices.Equal(names, want) {
		t.Fatalf("invoked %q, want %q", names, want)
	}
}

func TestUpgrade(t *testing.T) {
	w := mainWorld(t, "deploy_with_accessories")
	output := mainRun(w, "deploy_with_accessories", "upgrade", "-y")
	opts := invokeOptions("deploy_with_accessories.yml", Options{"confirmed": "true", "rolling": "false"}).Without("version")
	assertInvoked(t, w, invocation("proxy:upgrade", nil, opts), invocation("accessory:upgrade", []string{"all"}, opts))
	assertContains(t, output, "Upgrading all hosts...")
	assertContains(t, output, "Upgraded all hosts")
}

func TestUpgradeRolling(t *testing.T) {
	w := mainWorld(t, "deploy_with_accessories")
	output := mainRun(w, "deploy_with_accessories", "upgrade", "--rolling", "-y")
	opts := invokeOptions("deploy_with_accessories.yml", Options{"confirmed": "true", "rolling": "false"}).Without("version")
	counts := map[string]int{invocation("proxy:upgrade", nil, opts): 4, invocation("accessory:upgrade", []string{"all"}, opts): 3}
	for call, want := range counts {
		if got := countOf(w.invokes, call); got != want {
			t.Fatalf("%q invoked %d times, want %d:\n%s", call, got, want, strings.Join(w.invokes, "\n"))
		}
	}
	for _, host := range allHosts {
		assertContains(t, output, "Upgrading "+host+"...")
		assertContains(t, output, "Upgraded "+host)
	}
}

func countOf(items []string, item string) int {
	count := 0
	for _, each := range items {
		if each == item {
			count++
		}
	}
	return count
}
