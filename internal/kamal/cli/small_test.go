package cli

// Ports test/cli/lock_test.rb, prune_test.rb, and server_test.rb at Kamal 2.12.0.

import (
	"strings"
	"testing"
)

func lockWorld(t *testing.T) *world { return newWorld(t).fixture("deploy_with_accessories") }

const accessoriesConfig = "test/fixtures/deploy_with_accessories.yml"

func TestLockStatus(t *testing.T) {
	output := lockWorld(t).run("lock", "status", "-v", "-c", accessoriesConfig)
	assertContains(t, output, "Running /usr/bin/env stat .kamal/lock-app > /dev/null && cat .kamal/lock-app/details | base64 -d on 1.1.1.1")
}

func TestLockRelease(t *testing.T) {
	assertContains(t, lockWorld(t).run("lock", "release", "-v", "-c", accessoriesConfig), "Released the deploy lock")
}

func TestLockStatusWhenThereIsNoDeployLock(t *testing.T) {
	w := lockWorld(t).stubRemote(contains("base64 -d"), "", "cat: .kamal/lock-app/details: No such file or directory")
	assertContains(t, w.run("lock", "status", "-v", "-c", accessoriesConfig), "There is no deploy lock")
}

func TestLockReleaseWhenThereIsNoDeployLock(t *testing.T) {
	w := lockWorld(t).stubRemote(anything, "", "rm: cannot remove '.kamal/lock-app/details': No such file or directory")
	assertContains(t, w.run("lock", "release", "-v", "-c", accessoriesConfig), "There is no deploy lock")
}

func TestPruneAll(t *testing.T) {
	output := lockWorld(t).run("prune", "all", "-c", accessoriesConfig)
	assertContains(t, output, "tail -n +6")
	assertContains(t, output, "docker image prune --force --filter label=service=app")
}

func TestPruneImages(t *testing.T) {
	output := lockWorld(t).run("prune", "images", "-c", accessoriesConfig)
	assertContains(t, output, "docker image prune --force --filter label=service=app on 1.1.1.")
	assertContains(t, output, `docker image ls --filter label=service=app --format '{{.ID}} {{.Repository}}:{{.Tag}}' | grep -v -w "$(docker container ls -a --format '{{.Image}}\|' --filter label=service=app | tr -d '\n')dhh/app:<none>\|[^ ]*:latest\(-[^ ]*\)\?$" | while read image tag; do docker rmi $tag; done on 1.1.1.`)
}

func TestPruneContainers(t *testing.T) {
	const filters = `docker ps -q -a --filter label=service=app --filter label=destination= --filter status=created --filter status=exited --filter status=dead | tail -n +`
	assertContains(t, lockWorld(t).run("prune", "containers", "-c", accessoriesConfig), filters+"6 | while read container_id; do docker rm $container_id; done on 1.1.1.")
	assertContains(t, lockWorld(t).run("prune", "containers", "--retain", "10", "-c", accessoriesConfig), filters+"11 | while read container_id; do docker rm $container_id; done on 1.1.1.")
	assertContains(t, lockWorld(t).runFailing("prune", "containers", "--retain", "0", "-c", accessoriesConfig), "retain must be at least 1")
}

var allHosts = []string{"1.1.1.1", "1.1.1.2", "1.1.1.3", "1.1.1.4"}

func TestRunningACommandWithExec(t *testing.T) {
	for _, args := range [][]string{{"date"}, {"date", "-j"}} {
		command := strings.Join(args, " ")
		w := lockWorld(t).stubRemote(contains("date"), "Today", "")
		output := w.run(append(append([]string{"server", "exec"}, args...), "-c", accessoriesConfig)...)
		assertContains(t, output, "Running '"+command+"' on 1.1.1.1, 1.1.1.2, 1.1.1.3, 1.1.1.4...")
		for _, host := range allHosts {
			assertContains(t, output, "App Host: "+host+"\nToday")
		}
	}
}

func TestRunningACommandWithExecRawWritesStdoutVerbatim(t *testing.T) {
	w := lockWorld(t).stubRemote(equals("/usr/bin/env date"), "Today", "")
	output := w.run("server", "exec", "date", "--raw", "-c", accessoriesConfig)
	if output != strings.Repeat("Today", 4) {
		t.Fatalf("output = %q", output)
	}
}

func TestExecRawIsIncompatibleWithInteractive(t *testing.T) {
	assertContains(t, lockWorld(t).runFailing("server", "exec", "date", "--raw", "--interactive", "-c", accessoriesConfig), "ERROR (ArgumentError)")
}

// stubSetup stubs the lock and run directory commands, as stub_setup does.
func stubSetup(w *world) *world {
	for _, command := range []string{"/usr/bin/env mkdir -p .kamal", "/usr/bin/env mkdir .kamal/lock-app", "/usr/bin/env rm .kamal/lock-app/details"} {
		w.stubRemote(contains(command), "", "")
	}
	return w
}

func TestBootstrapAlreadyInstalled(t *testing.T) {
	w := stubSetup(lockWorld(t)).stubRemote(equals("docker -v"), "", "")
	if output := w.run("server", "bootstrap", "-c", accessoriesConfig); output != "Acquiring the deploy lock...\nReleasing the deploy lock..." {
		t.Fatalf("output = %q", output)
	}
}

func TestBootstrapInstallAsNonRootUser(t *testing.T) {
	w := stubSetup(lockWorld(t))
	output := w.runFailing("server", "bootstrap", "-c", accessoriesConfig)
	assertContains(t, output, "Docker is not installed on")
	assertContains(t, output, "and can't be automatically installed without having root access and either `wget` or `curl`")
}

func TestBootstrapInstallAsRootUser(t *testing.T) {
	w := stubSetup(lockWorld(t)).hooksExist().
		stubRemote(equals(`[ "${EUID:-$(id -u)}" -eq 0 ] || sudo -nl usermod >/dev/null`), "", "").
		stubRemote(equals(`[ "${EUID:-$(id -u)}" -eq 0 ]`), "", "")
	output := w.run("server", "bootstrap", "-c", accessoriesConfig)
	for _, host := range allHosts {
		assertContains(t, output, "Missing Docker on "+host+". Installing…")
	}
	assertHookRan(t, output, "docker-setup", 1)
}

func TestBootstrapInstallAsSudoNonRootUser(t *testing.T) {
	w := stubSetup(lockWorld(t)).hooksExist().
		stubRemote(equals(`[ "${EUID:-$(id -u)}" -eq 0 ] || sudo -nl usermod >/dev/null`), "", "").
		stubRemote(equals("kill -HUP $PPID"), "", "closed stream")
	output := w.run("server", "bootstrap", "-c", accessoriesConfig)
	for _, host := range allHosts {
		assertContains(t, output, "Missing Docker on "+host+". Installing…")
	}
	assertContains(t, output, "Session refreshed due to group change.")
}
