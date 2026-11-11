package cli

// Ports test/cli/registry_test.rb and the extract and print tests of
// test/cli/secrets_test.rb at Kamal 2.12.0. The fetch tests need the secrets
// adapters and come with them.

import (
	"strings"
	"testing"
)

const (
	localLogin   = `docker login -u \[REDACTED\] -p \[REDACTED\] as .*@localhost`
	remoteLogin  = `docker login -u \[REDACTED\] -p \[REDACTED\] on 1\.1\.1\.\d`
	localLogout  = `docker logout as .*@localhost`
	remoteLogout = `docker logout on 1\.1\.1\.\d`
)

func registryRun(t *testing.T, fixture string, args ...string) string {
	t.Helper()
	w := newWorld(t).fixture("deploy_" + fixture)
	return w.run(append(append([]string{"registry"}, args...), "-c", "test/fixtures/deploy_"+fixture+".yml")...)
}

func TestRegistrySetupAndLogin(t *testing.T) {
	cases := []struct {
		args          []string
		local, remote bool
	}{
		{[]string{"setup"}, true, true}, {[]string{"setup", "-L"}, false, true}, {[]string{"setup", "-R"}, true, false},
		{[]string{"login"}, true, true}, {[]string{"login", "-L"}, false, true}, {[]string{"login", "-R"}, true, false},
	}
	for _, c := range cases {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			output := registryRun(t, "with_accessories", c.args...)
			checkPresence(t, output, localLogin, c.local)
			checkPresence(t, output, remoteLogin, c.remote)
		})
	}
}

func TestRegistryRemoveAndLogout(t *testing.T) {
	cases := []struct {
		args          []string
		local, remote bool
	}{
		{[]string{"remove"}, true, true}, {[]string{"remove", "-L"}, false, true}, {[]string{"remove", "-R"}, true, false},
		{[]string{"logout"}, true, true}, {[]string{"logout", "-L"}, false, true}, {[]string{"logout", "-R"}, true, false},
	}
	for _, c := range cases {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			output := registryRun(t, "with_accessories", c.args...)
			checkPresence(t, output, localLogout, c.local)
			checkPresence(t, output, remoteLogout, c.remote)
		})
	}
}

func checkPresence(t *testing.T, output, pattern string, present bool) {
	t.Helper()
	if regexpMatch(pattern, output) != present {
		t.Fatalf("pattern %s present=%v, want %v:\n%s", pattern, !present, present, output)
	}
}

func TestRegistrySetupWithNoDocker(t *testing.T) {
	w := stubSetup(newWorld(t).fixture("deploy_with_accessories")).
		stubLocal(equals("docker --version && docker buildx version"), "", "command not found")
	assertContains(t, w.runFailing("registry", "setup", "-c", accessoriesConfig), "ERROR (Kamal::Cli::DependencyError): Docker is not installed locally")
}

func TestAllowRemoteLoginWithNoDocker(t *testing.T) {
	w := stubSetup(newWorld(t).fixture("deploy_with_accessories")).
		stubLocal(equals("docker --version && docker buildx version"), "", "command not found")
	w.run("registry", "setup", "--skip-local", "-c", accessoriesConfig)
}

func TestRegistrySetupAndRemoveLocalRegistry(t *testing.T) {
	assertMatches(t, registryRun(t, "with_local_registry", "setup"), `docker start kamal-docker-registry \|\| docker run --detach -p 127\.0\.0\.1:5000:5000 --name kamal-docker-registry registry:3 as .*@localhost`)
	assertMatches(t, registryRun(t, "with_local_registry", "remove"), `docker stop kamal-docker-registry && docker rm kamal-docker-registry as .*@localhost`)
}

func TestLoginAndLogoutWithLocalRegistryRaiseErrors(t *testing.T) {
	w := newWorld(t).fixture("deploy_with_local_registry")
	assertContains(t, w.runFailing("registry", "login", "-c", "test/fixtures/deploy_with_local_registry.yml"), "Cannot use login command with a local registry. Use `kamal registry setup` instead.")
	w = newWorld(t).fixture("deploy_with_local_registry")
	assertContains(t, w.runFailing("registry", "logout", "-c", "test/fixtures/deploy_with_local_registry.yml"), "Cannot use logout command with a local registry. Use `kamal registry remove` instead.")
}

func TestSecretsExtract(t *testing.T) {
	for _, secrets := range []string{`{"foo":"oof", "bar":"rab", "baz":"zab"}`, `{"abc/foo":"oof", "bar":"rab", "baz":"zab"}`} {
		if got := lockWorld(t).run("secrets", "extract", "foo", secrets, "-c", accessoriesConfig); got != "oof" {
			t.Errorf("extract from %s = %q", secrets, got)
		}
	}
}

func TestSecretsPrint(t *testing.T) {
	w := lockWorld(t)
	w.files[".kamal/secrets"] = "SECRET1=ABC\nSECRET2=${SECRET1}DEF\n"
	if got := w.run("secrets", "print", "-c", accessoriesConfig); got != "SECRET1=ABC\nSECRET2=ABCDEF" {
		t.Fatalf("print = %q", got)
	}
}
