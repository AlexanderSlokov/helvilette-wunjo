package main

import (
	"os"
	"strings"
	"testing"
)

func TestRunPrintsVersion(t *testing.T) {
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w
	status := run([]string{"version"})
	os.Stdout = stdout
	w.Close()
	buf := make([]byte, 64)
	n, _ := r.Read(buf)
	if status != 0 || strings.TrimSpace(string(buf[:n])) != "0.1.0" {
		t.Fatalf("status %d output %q", status, buf[:n])
	}
}
