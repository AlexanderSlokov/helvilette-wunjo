//go:build integration

// Package integration ports Kamal's integration suite: it starts Docker hosts, a
// registry, a load balancer, and an OTel collector with Docker Compose, and deploys
// the sample apps with the wunjo binary.
//
// Ports test/integration/integration_test.rb at Kamal 2.12.0. Run it with
// `make test-integration`.
package integration

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// repoRoot is the repository, which the deployer mounts at /wunjo.
var repoRoot, _ = filepath.Abs("../..")

// composeProject is stable per checkout, so `down` finds the stack of an earlier
// run, and unique across checkouts, so two suites can run at once.
var composeProject = func() string {
	sum := sha256.Sum256([]byte(repoRoot))
	return "wunjo-test-" + hex.EncodeToString(sum[:])[:8]
}()

var deployerSetup sync.Once

// TestMain builds the Linux wunjo binary and the Compose images once per run.
func TestMain(m *testing.M) {
	if err := buildBinary(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if out, err := compose("", "build"); err != nil {
		fmt.Fprintf(os.Stderr, "docker compose build failed: %v\n%s", err, out)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func buildBinary() error {
	build := exec.Command("go", "build", "-o", filepath.Join(repoRoot, "bin", "wunjo-integration"), "./cmd/wunjo")
	build.Dir = repoRoot
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+runtime.GOARCH)
	if out, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("go build failed: %v\n%s", err, out)
	}
	return nil
}

// compose runs `docker compose` for the stack of testID and returns its combined
// output.
func compose(testID string, args ...string) (string, error) {
	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
	cmd.Env = append(os.Environ(), "TEST_ID="+testID, "COMPOSE_PROJECT_NAME="+composeProject)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// stack is one running Compose stack and the app that a test deploys.
type stack struct {
	t         *testing.T
	testID    string
	app       string
	httpPort  string
	httpsPort string
}

// newStack starts a fresh stack, as IntegrationTest's setup does, and stops it
// when the test ends.
func newStack(t *testing.T) *stack {
	s := &stack{t: t, testID: randomHex(16), app: "app"}
	t.Cleanup(s.down)
	s.up()
	s.waitForHealthy(60 * time.Second)
	deployerSetup.Do(func() { s.deployer("/", "./setup.sh") })
	s.deployer("/", "sh", "-c", "rm -f /tmp/otel/*.json /tmp/kamal-deploy-logs/*")
	s.httpPort = s.publishedPort("load_balancer", "80")
	s.httpsPort = s.publishedPort("vm1", "443")
	return s
}

// up starts the stack and retries once, as compose_up_with_retry does.
func (s *stack) up() {
	if out, err := compose(s.testID, "up", "-d", "--no-build"); err == nil {
		return
	} else {
		s.t.Logf("compose up failed, retrying once: %v\n%s", err, out)
	}
	_, _ = compose(s.testID, "down", "-t", "0")
	s.mustCompose("up", "-d", "--no-build")
}

func (s *stack) down() {
	if s.t.Failed() && os.Getenv("DEBUG_CONTAINER_LOGS") != "" {
		for _, service := range []string{"deployer", "vm1", "vm2", "shared", "load_balancer", "registry"} {
			out, _ := compose(s.testID, "logs", "--no-color", service)
			s.t.Logf("Logs for %s:\n%s", service, out)
		}
	}
	_, _ = compose(s.testID, "down", "-t", "0")
}

func (s *stack) mustCompose(args ...string) string {
	s.t.Helper()
	out, err := compose(s.testID, args...)
	if err != nil {
		s.t.Fatalf("docker compose %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

// publishedPort returns the host port that Compose assigned, such as "32768".
func (s *stack) publishedPort(service, port string) string {
	out := strings.TrimSpace(s.mustCompose("port", service, port))
	first := strings.SplitN(out, "\n", 2)[0]
	return first[strings.LastIndex(first, ":")+1:]
}

// containerStatus is one line of `docker compose ps --format json`.
type containerStatus struct {
	Service  string `json:"Service"`
	State    string `json:"State"`
	Health   string `json:"Health"`
	ExitCode int    `json:"ExitCode"`
}

func (s *stack) waitForHealthy(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for {
		statuses := s.containerStatuses()
		unhealthy := notHealthy(statuses)
		if len(unhealthy) == 0 {
			return
		}
		if broken := brokenContainers(statuses); len(broken) > 0 {
			s.t.Fatalf("container hard error, a retry does not help: %+v", broken)
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("containers not healthy after %s: %+v", timeout, unhealthy)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *stack) containerStatuses() []containerStatus {
	var statuses []containerStatus
	for _, line := range strings.Split(strings.TrimSpace(s.mustCompose("ps", "-a", "--format", "json")), "\n") {
		var status containerStatus
		if json.Unmarshal([]byte(line), &status) == nil {
			statuses = append(statuses, status)
		}
	}
	return statuses
}

func notHealthy(statuses []containerStatus) []containerStatus {
	var out []containerStatus
	for _, c := range statuses {
		if c.Health != "healthy" {
			out = append(out, c)
		}
	}
	return out
}

func brokenContainers(statuses []containerStatus) []containerStatus {
	var out []containerStatus
	for _, c := range statuses {
		if c.Health == "unhealthy" || c.State == "exited" || c.State == "dead" || c.State == "restarting" {
			out = append(out, c)
		}
	}
	return out
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
