//go:build integration

package integration

// Ports the command helpers of test/integration/integration_test.rb at Kamal 2.12.0:
// running commands in the deployer and on the hosts, and changing the sample apps.

import (
	"strings"
)

// deployerTry runs a command in the deployer and returns its output and error.
func (s *stack) deployerTry(workdir string, args ...string) (string, error) {
	return compose(s.testID, append([]string{"exec", "-T", "--workdir", workdir, "deployer"}, args...)...)
}

// deployer runs a command in the deployer and fails the test when it fails.
func (s *stack) deployer(workdir string, args ...string) string {
	s.t.Helper()
	out, err := s.deployerTry(workdir, args...)
	if err != nil {
		s.t.Fatalf("deployer %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

// wunjo runs wunjo in the app directory and returns its output without
// surrounding whitespace, as the Ruby suite's `kamal` with capture: true does.
func (s *stack) wunjo(args ...string) string {
	s.t.Helper()
	return strings.TrimSpace(s.deployer("/"+s.app, append([]string{"wunjo"}, args...)...))
}

// wunjoTry runs wunjo and returns the output even when the command fails.
func (s *stack) wunjoTry(args ...string) (string, error) {
	out, err := s.deployerTry("/"+s.app, append([]string{"wunjo"}, args...)...)
	return strings.TrimSpace(out), err
}

// onHost runs a shell command on a host container, such as "vm1".
func (s *stack) onHost(host, command string) string {
	s.t.Helper()
	return strings.TrimSpace(s.mustCompose("exec", "-T", host, "sh", "-c", command))
}

func (s *stack) latestAppVersion() string {
	s.t.Helper()
	return strings.TrimSpace(s.deployer("/"+s.app, "git", "rev-parse", "HEAD"))
}

func (s *stack) updateAppRev() string {
	s.t.Helper()
	s.deployer("/", "./update_app_rev.sh", s.app)
	return s.latestAppVersion()
}

func (s *stack) breakApp() string {
	s.t.Helper()
	s.deployer("/", "./break_app.sh", s.app)
	return s.latestAppVersion()
}

// assertHooksRan removes the marker file of each hook, which the hook created.
func (s *stack) assertHooksRan(hooks ...string) {
	s.t.Helper()
	for _, hook := range hooks {
		file := "/tmp/" + s.testID + "/" + hook
		if got := strings.TrimSpace(s.deployer("/"+s.app, "rm", "-v", file)); got != "removed '"+file+"'" {
			s.t.Fatalf("hook %s did not run: %s", hook, got)
		}
	}
}

func (s *stack) containerRunning(host, name string) bool {
	s.t.Helper()
	return s.onHost(host, "docker ps --filter=name="+name+" | tail -n+2") != ""
}

func (s *stack) assertContainerRunning(host, name string) {
	s.t.Helper()
	if !s.containerRunning(host, name) {
		s.t.Fatalf("container %s is not running on %s", name, host)
	}
}

func (s *stack) assertContainerNotRunning(host, name string) {
	s.t.Helper()
	if s.containerRunning(host, name) {
		s.t.Fatalf("container %s is running on %s", name, host)
	}
}

func (s *stack) assertProxyRunning() { s.t.Helper(); s.assertContainerRunning("vm1", "kamal-proxy") }
func (s *stack) assertProxyNotRunning() {
	s.t.Helper()
	s.assertContainerNotRunning("vm1", "kamal-proxy")
}

// assertAppDirectoryRemoved checks the app directory on vm1. Kamal's helper lists
// ./kamal/apps/<app> in /work, which never exists, so it always passes. The port
// checks the directory that `remove` deletes, /root/.kamal/apps/<app>.
func (s *stack) assertAppDirectoryRemoved() {
	s.t.Helper()
	if got := s.onHost("vm1", "ls -d /root/.kamal/apps/"+s.app+" 2>/dev/null | wc -l"); got != "0" {
		s.t.Fatalf("/root/.kamal/apps/%s still exists on vm1", s.app)
	}
}

// grepLines returns the lines of text that contain part, as `| grep part` does.
func grepLines(text, part string) string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, part) {
			lines = append(lines, strings.TrimRight(line, "\r"))
		}
	}
	return strings.Join(lines, "\n")
}
