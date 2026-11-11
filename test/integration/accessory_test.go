//go:build integration

package integration

// Ports test/integration/accessory_test.rb at Kamal 2.12.0. The busybox accessory
// is on the web role, so it runs on vm1 and vm2.

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

var accessoryHosts = []string{"vm1", "vm2"}

func TestAccessoryBootStopStartRestartLogsRemove(t *testing.T) {
	s := newStack(t)
	s.wunjo("accessory", "boot", "busybox")
	s.assertAccessoryRunning("busybox", "1.36.0")
	assertMatches(t, s.onHost("vm1", "docker inspect custom-busybox --format '{{json .Mounts}}'"), `/data.*"RW":false`)
	assertMatches(t, s.onHost("vm1", "stat -c '%a %u:%g' /root/custom-busybox/etc/busybox.conf"), `640 1000:1000`)
	assertMatches(t, s.onHost("vm1", "stat -c '%a %u:%g' /root/custom-busybox/data"), `750 1000:1000`)

	s.wunjo("accessory", "stop", "busybox")
	s.assertAccessoryNotRunning("busybox")
	s.wunjo("accessory", "start", "busybox")
	s.assertAccessoryRunning("busybox", "1.36.0")
	s.wunjo("accessory", "restart", "busybox")
	s.assertAccessoryRunning("busybox", "1.36.0")

	assertMatches(t, s.wunjo("accessory", "logs", "busybox"), `Starting busybox\.\.\.`)
	assertMatches(t, s.wunjo("accessory", "boot", "busybox"), "Skipping booting `busybox` on vm1, vm2, a container already exists")

	// Rebooting onto the same image keeps it, as docker refuses to remove an image
	// that a container still references.
	assertMatches(t, s.wunjo("accessory", "reboot", "busybox"), `Kept busybox image sha256:[0-9a-f]+ on vm\d, docker declined to remove it`)
	s.assertAccessoryRunning("busybox", "1.36.0")
	for _, host := range accessoryHosts {
		assertMatches(t, s.onHost(host, "docker image ls busybox --format '{{.Repository}}:{{.Tag}}'"), `busybox:1\.36\.0`)
	}

	s.assertRebootSupersedesTheImage()
	s.wunjo("accessory", "remove", "busybox", "-y")
	s.assertAccessoryNotRunning("busybox")
}

// assertRebootSupersedesTheImage points the accessory at another image, so the
// reboot leaves the old image unreferenced and removes it on every host.
func (s *stack) assertRebootSupersedesTheImage() {
	s.t.Helper()
	superseded := map[string]string{}
	for _, host := range accessoryHosts {
		superseded[host] = s.onHost(host, "docker inspect custom-busybox --format '{{.Image}}'")
	}
	s.deployer("/app", "sed", "-i", "s|image: busybox:1.36.0|image: busybox:1.37.0|", "config/deploy.yml")
	s.wunjo("accessory", "reboot", "busybox")
	s.assertAccessoryRunning("busybox", "1.37.0")
	for host, imageID := range superseded {
		current := s.onHost(host, "docker inspect custom-busybox --format '{{.Image}}'")
		if current == imageID || strings.Contains(s.onHost(host, "docker image ls -aq --no-trunc"), imageID) {
			s.t.Fatalf("%s still has the superseded image %s", host, imageID)
		}
	}
}

func (s *stack) assertAccessoryRunning(name, version string) {
	s.t.Helper()
	assertMatches(s.t, s.wunjo("accessory", "details", name), `busybox:`+strings.ReplaceAll(version, ".", `\.`)+`   "sh -c 'echo \\"Start`)
}

// assertAccessoryNotRunning ignores the version, so it covers removal after the
// accessory moved to another image.
func (s *stack) assertAccessoryNotRunning(name string) {
	s.t.Helper()
	assertNoMatches(s.t, s.wunjo("accessory", "details", name), `busybox:\S+   "sh -c 'echo \\"Start`)
}

func TestProxiedAccessoryBootStopStartRestartRemove(t *testing.T) {
	s := newStack(t)
	s.app = "app_with_proxied_accessory"
	s.wunjo("proxy", "boot")

	s.wunjo("accessory", "boot", "netcat")
	s.assertAccessoryRunning("netcat", "1.36.0")
	s.assertNetcat("200")

	s.wunjo("accessory", "stop", "netcat")
	s.assertAccessoryNotRunning("netcat")
	s.assertNetcat("404")

	s.wunjo("accessory", "start", "netcat")
	s.assertAccessoryRunning("netcat", "1.36.0")
	s.assertNetcat("200")

	s.wunjo("accessory", "restart", "netcat")
	s.assertAccessoryRunning("netcat", "1.36.0")
	s.assertNetcat("200")

	s.wunjo("accessory", "remove", "netcat", "-y")
	s.assertAccessoryNotRunning("netcat")
	s.assertNetcat("404")
}

// assertNetcat waits up to 20 seconds for the proxied accessory to answer want.
func (s *stack) assertNetcat(want string) {
	s.t.Helper()
	client, url := &http.Client{Timeout: 15 * time.Second}, "http://127.0.0.1:"+s.httpPort+"/up"
	deadline := time.Now().Add(20 * time.Second)
	resp := s.get(client, url, "netcat")
	for resp.code != want && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		resp = s.get(client, url, "netcat")
	}
	s.debugResponseCode(resp, want)
	if resp.code != want {
		s.t.Fatalf("netcat answered %s, want %s", resp.code, want)
	}
}
