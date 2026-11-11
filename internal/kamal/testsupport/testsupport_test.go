package testsupport

import (
	"strings"
	"testing"
)

func TestReadFixtureReplacesArchHelper(t *testing.T) {
	content, err := ReadFixture("deploy_with_remote_builder.yml")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(content, "ArchHelper") || !strings.Contains(content, `ENV["WUNJO_TEST_REMOTE_ARCH"]`) {
		t.Fatalf("fixture still calls ArchHelper:\n%s", content)
	}
}

func TestFakeEnvLookup(t *testing.T) {
	env := FakeEnv{"A": ""}
	if value, ok := env.Lookup("A"); !ok || value != "" {
		t.Fatal("set empty variable not found")
	}
	if _, ok := env.Lookup("B"); ok {
		t.Fatal("unset variable found")
	}
}

func TestFakeFiles(t *testing.T) {
	files := FakeFiles{"a": "x"}
	if content, ok, _ := files.Read("a"); !ok || string(content) != "x" || !files.Exists("a") || files.Exists("b") {
		t.Fatal("FakeFiles lookups are wrong")
	}
	if _, err := files.ReadFile("b"); err == nil {
		t.Fatal("ReadFile of a missing file did not fail")
	}
}
