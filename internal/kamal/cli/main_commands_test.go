package cli

// Ports the rollback, config, init, docs, version, and alias tests of
// test/cli/main_test.rb at Kamal 2.12.0. The config tests compare the whole output
// with the output of Ruby Kamal 2.12.0 in testdata/config, which checks more than
// the fields that the Ruby tests read back.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/testsupport"
)

func TestRollbackBadVersion(t *testing.T) {
	output := mainRun(newWorld(t).fixture("deploy_simple"), "deploy_simple", "rollback", "nonsense")
	assertContains(t, output, "docker container ls --all --filter 'name=^app-web-nonsense$' --quiet")
	assertContains(t, output, "The app version 'nonsense' is not available as a container")
}

// stubRollbackRole stubs the container lookups of one role: no container under
// the new name yet, version-to-rollback running.
func stubRollbackRole(w *world, role string) {
	lookup := equals("docker container ls --all --filter 'name=^app-" + role + "-123$' --quiet")
	w.hosts.stubs = append(w.hosts.stubs,
		stub{match: lookup, only: "lenient"},
		stub{match: lookup, only: "strict", output: "version-to-rollback\n"},
		stub{match: contains("while read line; do echo ${line#app-" + role + "-}; done"), only: "lenient", output: "version-to-rollback\n"})
}

func TestRollbackGoodVersion(t *testing.T) {
	w := lockWorld(t).hooksExist()
	w.files[".kamal/secrets"] = "MYSQL_ROOT_PASSWORD=secret"
	w.stubCapture(contains("'name=^app-workers-123$' --quiet | xargs docker inspect"), "running")
	stubRollbackRole(w, "web")
	stubRollbackRole(w, "workers")
	output := w.run("rollback", "--verbose", "123", "-c", accessoriesConfig)
	assertHookRan(t, output, "pre-deploy", 1)
	assertContains(t, output, "docker tag dhh/app:123 dhh/app:latest")
	assertContains(t, output, "docker run --detach --restart unless-stopped --name app-web-123")
	assertContains(t, output, "docker container ls --all --filter 'name=^app-web-version-to-rollback$' --quiet | xargs docker stop")
	assertHookRan(t, output, "post-deploy", 1)
}

func TestRollbackWithoutOldVersion(t *testing.T) {
	w := newWorld(t).fixture("deploy_simple")
	lookup := equals("docker container ls --all --filter 'name=^app-web-123$' --quiet")
	w.hosts.stubs = append(w.hosts.stubs, stub{match: lookup, only: "lenient"}, stub{match: lookup, only: "strict", output: "123"},
		stub{match: contains("while read line; do echo ${line#app-web-}; done"), only: "lenient"})
	output := mainRun(w, "deploy_simple", "rollback", "123")
	assertContains(t, output, "docker run --detach --restart unless-stopped --name app-web-123")
	assertNotContains(t, output, "docker stop")
}

func TestAudit(t *testing.T) {
	output := mainRun(newWorld(t).fixture("deploy_simple"), "deploy_simple", "audit")
	assertContains(t, output, "tail -n 50 .kamal/app-audit.log on 1.1.1.1")
	assertContains(t, output, "App Host: 1.1.1.1")
}

func TestConfig(t *testing.T) {
	cases := map[string][]string{
		"deploy_simple":                    {"deploy_simple"},
		"deploy_with_roles":                {"deploy_with_roles"},
		"deploy_primary_web_role_override": {"deploy_primary_web_role_override"},
		"deploy_for_dest.world":            {"deploy_for_dest", "deploy_for_dest.world"},
	}
	for golden, fixtures := range cases {
		w := newWorld(t)
		for _, fixture := range fixtures {
			w.fixture(fixture)
		}
		args := []string{"config"}
		if len(fixtures) > 1 {
			args = append(args, "-d", "world")
		}
		want, err := os.ReadFile(filepath.Join("testdata", "config", golden+".yml"))
		if err != nil {
			t.Fatal(err)
		}
		if got := mainRun(w, fixtures[0], args...); got != strings.TrimSpace(string(want)) {
			t.Errorf("config of %s:\n%s\nwant the output of Ruby Kamal:\n%s", golden, got, want)
		}
	}
}

const trimmedServers = "service: app\nimage: dhh/app\nservers:\n  - \"1.1.1.1\"\n%s\n  - \"1.1.1.2\"\n%s\n" +
	"registry:\n  username: user\n  password: pw\nbuilder:\n  arch: amd64\n"

// The Ruby test renders the servers inside `<% if true -%>`. The ERB subset of
// Wunjo has no statement tags, so this port trims comment tags, and the
// conditional fails with a message that names the tag. See "Known differences
// from Kamal" in docs/plans/port-kamal-to-go.md.
func TestConfigWithBlankLineTrimming(t *testing.T) {
	w := newWorld(t)
	w.files["/work/deploy.yml"] = strings.Replace(strings.Replace(trimmedServers, "%s", "<%# first -%>", 1), "%s", "<%# last -%>", 1)
	assertContains(t, w.run("config", "-c", "/work/deploy.yml"), ":hosts:\n- 1.1.1.1\n- 1.1.1.2\n")
	w = newWorld(t)
	w.files["/work/deploy.yml"] = strings.Replace(strings.Replace(trimmedServers, "%s", "<% if true -%>", 1), "%s", "<% end -%>", 1)
	assertContains(t, w.runFailing("config", "-c", "/work/deploy.yml"), "unsupported ERB expression: if true")
}

func TestConfigWithDestinationBlankLineTrimming(t *testing.T) {
	w := newWorld(t)
	w.files["/work/deploy.yml"] = "service: app\nimage: dhh/app\nservers:\n  - \"1.1.1.1\"\nregistry:\n  username: user\n  password: pw\nbuilder:\n  arch: amd64\n"
	w.files["/work/deploy.world.yml"] = "servers:\n  - \"2.2.2.2\"\n<%# first -%>\n  - \"2.2.2.3\"\n<%# last -%>\n"
	assertContains(t, w.run("config", "-d", "world", "-c", "/work/deploy.yml"), ":hosts:\n- 2.2.2.2\n- 2.2.2.3\n")
}

func TestInit(t *testing.T) {
	w := newWorld(t)
	output := mainRun(w, "deploy_simple", "init")
	assertContains(t, output, "Created configuration file in config/deploy.yml")
	assertContains(t, output, "Created .kamal/secrets file")
	assertContains(t, w.files["/k/config/deploy.yml"], "service: my-app")
	assertContains(t, w.files["/k/.kamal/secrets"], "KAMAL_REGISTRY_PASSWORD=$KAMAL_REGISTRY_PASSWORD")
}

func TestInitWithExistingConfig(t *testing.T) {
	w := newWorld(t)
	mainRun(w, "deploy_simple", "init")
	w.out.reset()
	output := mainRun(w, "deploy_simple", "init")
	assertContains(t, output, "Config file already exists in config/deploy.yml (remove first to create a new one)")
	assertNotContains(t, output, "Added .kamal/secrets")
}

func TestInitWithBundleOption(t *testing.T) {
	output := mainRun(newWorld(t), "deploy_simple", "init", "--bundle")
	for _, want := range []string{"Created configuration file in config/deploy.yml", "Created .kamal/secrets file",
		"Adding Kamal to Gemfile and bundle", "bundle add kamal", "bundle binstubs kamal", "Created binstub file in bin/kamal"} {
		assertContains(t, output, want)
	}
}

func TestInitWithBundleOptionAndExistingBinstub(t *testing.T) {
	w := newWorld(t)
	for _, path := range []string{"/k/config/deploy.yml", "/k/.kamal/secrets", "/k/.kamal/hooks", "/k/bin/kamal"} {
		w.files[path] = ""
	}
	output := mainRun(w, "deploy_simple", "init", "--bundle")
	assertContains(t, output, "Config file already exists in config/deploy.yml (remove first to create a new one)")
	assertContains(t, output, "Binstub already exists in bin/kamal (remove first to create a new one)")
}

func TestRemoveWithConfirmation(t *testing.T) {
	w := lockWorld(t)
	w.files[".kamal/secrets"] = "MYSQL_ROOT_PASSWORD=secret"
	output := w.run("remove", "-y", "-c", accessoriesConfig)
	for _, want := range []string{
		"docker container stop kamal-proxy", "docker container prune --force --filter label=org.opencontainers.image.title=kamal-proxy",
		"docker image prune --all --force --filter label=org.opencontainers.image.title=kamal-proxy",
		"docker container prune --force --filter label=service=app",
		"docker image prune --all --force --filter label=service=app", "/usr/bin/env rm -r .kamal/apps/app",
		"docker container stop app-mysql", "docker container prune --force --filter label=service=app-mysql", "docker image rm --force mysql", "rm -rf app-mysql",
		"docker container stop app-redis", "docker container prune --force --filter label=service=app-redis", "docker image rm --force redis", "rm -rf app-redis",
		"docker logout",
	} {
		assertContains(t, output, want)
	}
	// The Ruby pattern is a regular expression, so "|" matches either side.
	assertMatches(t, output, `docker ps --quiet --filter label=service=app | xargs docker stop`)
}

func TestDocs(t *testing.T) {
	cases := map[string][]string{"# Kamal Configuration": {"docs"}, "# Accessories": {"docs", "accessory"}, "No documentation found for foo": {"docs", "foo"}}
	for want, args := range cases {
		assertContains(t, mainRun(newWorld(t), "deploy_simple", args...), want)
	}
}

func TestVersion(t *testing.T) {
	if got := newWorld(t).run("version"); got != Version {
		t.Fatalf("version = %q, want %q", got, Version)
	}
}

func TestRunAnAliasForDetails(t *testing.T) {
	w := mainWorld(t, "deploy_with_aliases")
	mainRun(w, "deploy_with_aliases", "info")
	assertInvokedNames(t, w, "proxy:details", "app:details", "accessory:details all")
}

func TestRunAnAliasForAConsole(t *testing.T) {
	cases := []struct {
		args       []string
		want, host string
	}{
		{[]string{"console"}, "docker exec app-console-999 bin/console on 1.1.1.5", "1.1.1.5"},
		{[]string{"console", "-r", "workers"}, "docker exec app-workers-999 bin/console on 1.1.1.3", "1.1.1.3"},
		{[]string{"exec", "bin/job"}, "docker exec app-console-999 bin/job on 1.1.1.5", "1.1.1.5"},
		{[]string{"rails", "db:migrate:status"}, "docker exec app-console-999 rails db:migrate:status on 1.1.1.5", "1.1.1.5"},
	}
	for _, c := range cases {
		output := mainRun(newWorld(t).fixture("deploy_with_aliases"), "deploy_with_aliases", c.args...)
		assertContains(t, output, c.want)
		assertContains(t, output, "App Host: "+c.host)
		assertNotContains(t, output, "App Host: 1.1.1.4")
	}
}

// configWorld copies deploy.yml, deploy2.yml, and deploy.elsewhere.yml into
// config/, as with_config_files does.
func configWorld(t *testing.T) *world {
	w := newWorld(t)
	for _, name := range []string{"deploy.yml", "deploy2.yml", "deploy.elsewhere.yml"} {
		content, err := testsupport.ReadFixture(name)
		if err != nil {
			t.Fatal(err)
		}
		w.files["/k/config/"+name] = content
	}
	return w
}

func TestSwitchConfigFileWithAnAlias(t *testing.T) {
	assertContains(t, configWorld(t).run("other_config"), ":service_with_version: app2-999")
	assertContains(t, configWorld(t).run("other_destination_config"), ":service_with_version: app3-999")
}

func TestRunAnAliasWithRequireDestination(t *testing.T) {
	w := mainWorld(t, "deploy_for_required_dest").fixture("deploy_for_required_dest.world")
	mainRun(w, "deploy_for_required_dest", "world_deploy")
	opts := invokeOptions("deploy_for_required_dest.yml", Options{"destination": "world"})
	assertInvoked(t, w, deployInvocations("build:deliver", opts)...)
}

func TestRunOnPrimaryViaAlias(t *testing.T) {
	output := mainRun(newWorld(t).fixture("deploy_with_aliases"), "deploy_with_aliases", "primary_details")
	assertContains(t, output, "App Host: 1.1.1.1")
	assertNotContains(t, output, "App Host: 1.1.1.2")
}

// The expected tree is the output of `kamal tree` from Ruby Kamal 2.12.0.
func TestTree(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "tree.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := newWorld(t).run("tree"); got != strings.TrimSpace(string(want)) {
		t.Fatalf("tree:\n%s\nwant the output of Ruby Kamal:\n%s", got, want)
	}
}
