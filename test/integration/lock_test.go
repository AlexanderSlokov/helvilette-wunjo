//go:build integration

package integration

// Ports test/integration/lock_test.rb at Kamal 2.12.0.

import (
	"regexp"
	"strings"
	"testing"
)

func TestLockAcquireReleaseStatus(t *testing.T) {
	s := newStack(t)
	s.wunjo("lock", "acquire", "-m", "Integration Tests")

	status := s.wunjo("lock", "status")
	if !regexp.MustCompile(`(?s)Locked by: Deployer at .*\nVersion: ` + s.latestAppVersion() + `\nMessage: Integration Tests`).MatchString(status) {
		t.Fatalf("lock status:\n%s", status)
	}

	out, err := s.wunjoTry("deploy")
	if err == nil || !strings.Contains(out, "Deploy lock found. Run 'kamal lock help' for more information") {
		t.Fatalf("deploy with a held lock: err %v\n%s", err, out)
	}

	s.wunjo("lock", "release")
	if status := s.wunjo("lock", "status"); !strings.Contains(status, "There is no deploy lock") {
		t.Fatalf("lock status after release:\n%s", status)
	}
}
