//go:build integration

package integration

// Ports test/integration/proxy_test.rb and test/integration/broken_deploy_test.rb
// at Kamal 2.12.0.

import (
	"testing"
)

func TestProxyBootRebootStopStartRestartLogsRemove(t *testing.T) {
	s := newStack(t)
	s.app = "app_with_roles"
	s.wunjo("proxy", "boot")
	s.assertProxyRunning()

	output := s.wunjo("proxy", "reboot", "-y", "--verbose")
	s.assertProxyRunning()
	s.assertHooksRan("pre-proxy-reboot", "post-proxy-reboot")
	assertMatches(t, output, `Rebooting kamal-proxy on vm1,vm2\.\.\.`, `Rebooted kamal-proxy on vm1,vm2`)

	output = s.wunjo("proxy", "reboot", "--rolling", "-y", "--verbose")
	s.assertProxyRunning()
	s.assertHooksRan("pre-proxy-reboot", "post-proxy-reboot")
	assertMatches(t, output, `Rebooting kamal-proxy on vm1\.\.\.`, `Rebooted kamal-proxy on vm1`,
		`Rebooting kamal-proxy on vm2\.\.\.`, `Rebooted kamal-proxy on vm2`)

	s.wunjo("proxy", "boot")
	s.assertProxyRunning()
	// booting a stopped proxy works
	s.wunjo("proxy", "stop")
	s.assertProxyNotRunning()
	s.wunjo("proxy", "boot")
	s.assertProxyRunning()

	s.wunjo("proxy", "stop")
	s.assertProxyNotRunning()
	s.wunjo("proxy", "start")
	s.assertProxyRunning()
	s.wunjo("proxy", "restart")
	s.assertProxyRunning()

	assertMatches(t, s.wunjo("proxy", "logs"), `No previous state to restore`)
	s.wunjo("proxy", "remove")
	s.assertProxyNotRunning()
}

func TestDeployingABadImage(t *testing.T) {
	s := newStack(t)
	s.app = "app_with_roles"
	first := s.latestAppVersion()
	s.wunjo("deploy")
	s.assertAppIsUp(first)
	s.assertContainerRunning("vm3", "app_with_roles-workers-"+first)

	second := s.breakApp()
	output, err := s.wunjoTry("deploy")
	if err == nil {
		t.Fatalf("deploying a broken app succeeded:\n%s", output)
	}
	assertMatches(t, output, `Waiting for the first healthy web container before booting workers on vm3\.\.\.`,
		`First web container is unhealthy on vm[12], not booting any other roles`,
		`First web container is unhealthy, not booting workers on vm3`,
		`nginx: \[emerg\] unexpected end of file, expecting ";" or "}" in /etc/nginx/conf\.d/default\.conf:2`)
	s.assertAppIsUp(first)
	s.assertContainerRunning("vm3", "app_with_roles-workers-"+first)
	s.assertContainerNotRunning("vm3", "app_with_roles-workers-"+second)
}
