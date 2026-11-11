package localshell

import (
	"io"
	"testing"
)

func TestSystemRunCapturesStdoutAndStatus(t *testing.T) {
	shell := System{Stderr: io.Discard}
	ok, err := shell.Run("echo hello")
	if err != nil || ok.Stdout != "hello\n" || !ok.Success {
		t.Fatalf("echo: %+v, %v", ok, err)
	}
	failed, err := shell.Run("echo partial; exit 3")
	if err != nil || failed.Stdout != "partial\n" || failed.Success {
		t.Fatalf("exit 3: %+v, %v", failed, err)
	}
}

func TestFakeRecordsAndRejectsUnknown(t *testing.T) {
	fake := &Fake{Results: map[string]Result{"whoami": {Stdout: "deployer\n", Success: true}}}
	if result, err := fake.Run("whoami"); err != nil || result.Stdout != "deployer\n" {
		t.Fatalf("whoami: %+v, %v", result, err)
	}
	if _, err := fake.Run("rm -rf /"); err == nil {
		t.Fatal("unknown command did not fail")
	}
	if len(fake.Calls) != 2 {
		t.Fatalf("calls = %v", fake.Calls)
	}
}
