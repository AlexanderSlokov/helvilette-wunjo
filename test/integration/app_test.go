//go:build integration

package integration

// Ports test/integration/app_test.rb at Kamal 2.12.0.

import (
	"regexp"
	"strings"
	"testing"
)

func TestAppStopStartBootLogsImagesContainersExecRemove(t *testing.T) {
	s := newStack(t)
	s.wunjo("deploy")
	s.assertAppIsUp("")

	s.wunjo("app", "stop")
	s.assertAppNotFound()
	s.wunjo("app", "start")
	s.waitForAppToBeUp()

	assertMatches(t, s.wunjo("app", "boot", "--verbose"), `Booting app on vm1,vm2\.\.\.`, `Booted app on vm1,vm2\.\.\.`)
	s.waitForAppToBeUp()

	version := s.latestAppVersion()
	assertMatches(t, s.wunjo("app", "logs"), `App Host: vm1`, `App Host: vm2`, `GET /version HTTP/1\.1`)
	assertMatches(t, s.wunjo("app", "images"), `App Host: vm1`, `App Host: vm2`, `localhost:5000/app[:\s]+`+version, `localhost:5000/app[:\s]+latest`)
	assertMatches(t, s.wunjo("app", "containers"), `App Host: vm1`, `App Host: vm2`, `localhost:5000/app:`+version, `localhost:5000/app:latest`)
	assertMatches(t, s.wunjo("app", "exec", "ps"), `App Host: vm1`, `App Host: vm2`, `1 root      0:\d\d ps`)
	assertMatches(t, s.wunjo("app", "exec", "--reuse", "ps"), `App Host: vm2`, `App Host: vm1`, `1 root      0:\d\d nginx`)

	md5 := regexp.MustCompile(`[0-9a-f]{32}`)
	expected := md5.FindString(s.deployer("/app", "sh", "-c", "head -c 512 /dev/zero | md5sum"))
	piped := md5.FindString(s.deployer("/app", "sh", "-c", "wunjo app exec --reuse --raw --primary 'head -c 512 /dev/zero' | md5sum"))
	if piped != expected {
		t.Fatalf("raw exec md5 = %s, want %s", piped, expected)
	}

	s.wunjo("app", "maintenance")
	s.assertAppInMaintenance("")
	s.wunjo("app", "live")
	s.assertAppIsUp("")

	s.wunjo("app", "remove")
	s.assertAppNotFound()
	s.assertAppDirectoryRemoved()
}

func TestAppParallelRoles(t *testing.T) {
	s := newStack(t)
	s.app = "app_with_parallel_roles"
	version := s.latestAppVersion()
	s.wunjo("deploy")
	s.assertAppIsUp(version)
	s.assertContainerRunning("vm1", "app_with_parallel_roles-web-"+version)
	s.assertContainerRunning("vm1", "app_with_parallel_roles-workers-"+version)
	s.assertContainerRunning("vm2", "app_with_parallel_roles-web-"+version)

	s.wunjo("app", "stop")
	s.assertAppNotFound()
	s.wunjo("app", "start")
	s.waitForAppToBeUp()

	assertMatches(t, s.wunjo("app", "logs"), `role=web.* on vm1`, `role=web.* on vm2`, `role=workers.* on vm1`)
	// images and containers run once per host, and exec once per role per host
	assertHostCount(t, s.wunjo("app", "images"), 2)
	containers := s.wunjo("app", "containers")
	assertMatches(t, containers, `app_with_parallel_roles-web`, `app_with_parallel_roles-workers`)
	assertHostCount(t, containers, 2)
	exec := s.wunjo("app", "exec", "hostname")
	assertMatches(t, exec, `app_with_parallel_roles-web-exec-.* on vm1`, `app_with_parallel_roles-web-exec-.* on vm2`, `app_with_parallel_roles-workers-exec-.* on vm1`)
	assertHostCount(t, exec, 3)

	s.wunjo("app", "maintenance")
	s.assertAppInMaintenance("")
	s.wunjo("app", "live")
	s.assertAppIsUp("")
}

func assertHostCount(t *testing.T, output string, want int) {
	t.Helper()
	if got := strings.Count(output, "App Host:"); got != want {
		t.Fatalf("App Host lines = %d, want %d:\n%s", got, want, output)
	}
}

func TestAppCustomErrorPages(t *testing.T) {
	s := newStack(t)
	s.app = "app_with_roles"
	s.wunjo("deploy")
	s.assertAppIsUp("")

	s.wunjo("app", "maintenance")
	s.assertAppInMaintenance("Custom Maintenance Page")

	s.wunjo("app", "live")
	s.wunjo("app", "maintenance", "--message", "Testing Maintence Mode")
	s.assertAppInMaintenance("Custom Maintenance Page: Testing Maintence Mode")

	s.updateAppRev()
	s.wunjo("redeploy")
	s.wunjo("app", "maintenance")
	s.assertAppInMaintenance("Custom Maintenance Page")
}
