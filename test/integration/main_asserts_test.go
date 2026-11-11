//go:build integration

package integration

// Ports the private assertions of test/integration/main_test.rb at Kamal 2.12.0.

import (
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/cli"
)

// assertOtelLogs checks the events and lines of deploy, redeploy, and rollback.
func (s *stack) assertOtelLogs() {
	s.t.Helper()
	found := s.waitForOtelEvents(6)
	s.assertOtelResource()
	for _, command := range []string{"deploy", "redeploy", "rollback"} {
		start := findEvent(found, "kamal.start", command)
		complete := findEvent(found, "kamal.complete", command)
		if start == nil || complete == nil {
			s.t.Fatalf("no start or complete event for %s in %+v", command, found)
		}
		if start.attr("deployment.id") == "" || start.attr("deployment.name") != command+" app" || complete.attr("deployment.status") != "succeeded" {
			s.t.Fatalf("deployment attributes of %s: start %+v, complete %+v", command, start.Attributes, complete.Attributes)
		}
	}
	if starts := countEvents(found, "kamal.start"); starts != 3 {
		s.t.Fatalf("kamal.start events = %d, want 3", starts)
	}
	s.assertOtelLines(s.otelRecords())
}

func (s *stack) assertOtelResource() {
	s.t.Helper()
	attrs := s.otelResourceAttributes()
	want := map[string]string{"service.name": "kamal", "service.namespace": "app", "service.version": cli.Version}
	for key, value := range want {
		if got := attrOf(attrs, key); got != value {
			s.t.Fatalf("resource %s = %q, want %q", key, got, value)
		}
	}
	for _, key := range []string{"kamal.run_id", "kamal.performer", "kamal.deploy_version"} {
		if attrOf(attrs, key) == "" {
			s.t.Fatalf("resource lacks %s: %+v", key, attrs)
		}
	}
}

// assertOtelLines checks the host-tagged command output among the records.
func (s *stack) assertOtelLines(records []otlpRecord) {
	s.t.Helper()
	hosts, streams, lines, docker := map[string]bool{}, map[string]bool{}, 0, false
	for _, r := range records {
		hosts[r.attr("server.address")] = true
		streams[r.attr("log.iostream")] = true
		if r.EventName == "" {
			lines++
			docker = docker || regexp.MustCompile(`Running docker`).MatchString(*r.Body.StringValue)
		}
	}
	if !hosts["vm1"] || !hosts["vm2"] || !streams["stdout"] || lines <= 10 || !docker {
		s.t.Fatalf("records: hosts %v, streams %v, %d lines, docker lines %v", hosts, streams, lines, docker)
	}
}

func findEvent(found []otlpRecord, name, command string) *otlpRecord {
	for i := range found {
		if found[i].EventName == name && found[i].attr("kamal.command") == command {
			return &found[i]
		}
	}
	return nil
}

func countEvents(found []otlpRecord, name string) int {
	count := 0
	for _, r := range found {
		if r.EventName == name {
			count++
		}
	}
	return count
}

func (s *stack) assertEnvs(version string) {
	s.t.Helper()
	want := []struct{ vm, key, value string }{
		{"vm1", "KAMAL_HOST", "vm1"}, {"vm1", "CLEAR_TOKEN", "4321"}, {"vm1", "HOST_TOKEN", "abcd"},
		{"vm1", "SECRET_TOKEN", `1234 with "中文"`}, {"vm1", "CLEAR_TAG", ""}, {"vm1", "SECRET_TAG", ""},
		{"vm2", "CLEAR_TAG", "tagged"}, {"vm2", "SECRET_TAG", "TAGME"},
		{"vm2", "INTERPOLATED_SECRET1", "1TERCES_DETALOPRETNI"}, {"vm2", "INTERPOLATED_SECRET2", "2TERCES_DETALOPRETNI"},
		{"vm2", "INTERPOLATED_SECRET3", "文中_DETALOPRETNI"}, {"vm2", "INTERPOLATED_SECRET4", ")(_DETALOPRETNI"},
	}
	for _, w := range want {
		got := grepLines(s.onHost(w.vm, "docker exec "+s.app+"-web-"+version+" env"), w.key)
		expected := w.key + "=" + w.value
		if w.value == "" {
			expected = ""
		}
		if got != expected {
			s.t.Fatalf("%s on %s: env has %q, want %q", w.key, w.vm, got, expected)
		}
	}
}

func (s *stack) assertHookEnvVariables(output, version string) {
	s.t.Helper()
	assertMatches(s.t, output, "KAMAL_VERSION="+version, "KAMAL_SERVICE=app", "KAMAL_SERVICE_VERSION=app@"+version[:7],
		"KAMAL_COMMAND=app", "KAMAL_PERFORMER=deployer@example.com", `KAMAL_RECORDED_AT=\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ`,
		"KAMAL_HOSTS=vm1,vm2", "KAMAL_ROLES=web")
}

// assertHookOutput checks hooks_output: pre-deploy is verbose, pre-build quiet,
// and post-deploy shows Running and hides its output.
func assertHookOutput(t *testing.T, output string) {
	t.Helper()
	assertMatches(t, output, `Running.*pre-deploy`, `Deployed!`, `Running.*post-deploy`)
	assertNoMatches(t, output, `Running.*pre-build`, `About to build and push`, `Finished deploy!`)
}

func (s *stack) assertAccumulatedAssets(versions ...string) {
	s.t.Helper()
	client := &http.Client{Timeout: 15 * time.Second}
	for _, path := range append(versions, ".hidden") {
		resp := s.get(client, "http://"+appHost(s.app)+":"+s.httpPort+"/versions/"+path, "")
		if resp.code != "200" {
			s.t.Fatalf("/versions/%s answered %s, want 200", path, resp.code)
		}
	}
}

func (s *stack) assertAssetVolumeReadOnly(version string) {
	s.t.Helper()
	mounts := s.onHost("vm1", "docker inspect app-web-"+version+" --format '{{json .Mounts}}'")
	assertMatches(s.t, mounts, `/usr/share/nginx/html/versions.*"RW":false`)
}
