//go:build integration

package sshexec

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
)

// TestRunsACommandOnARealHostThroughTheSSHAgent starts sshd in Docker, loads a new
// key into a private ssh-agent, and runs a command with the OpenSSH backend. It
// needs Docker, ssh, ssh-agent, ssh-keygen, and ssh-add. Run it with
// `make test-integration`.
func TestRunsACommandOnARealHostThroughTheSSHAgent(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "id_ed25519")
	run(t, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key)
	startAgent(t, dir, key)
	pub, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	port := startSSHD(t, strings.TrimSpace(string(pub)))
	config := filepath.Join(dir, "ssh_config")
	writeFile(t, config, "Host *\n  UserKnownHostsFile "+filepath.Join(dir, "known_hosts")+"\n  IdentityFile none\n")

	// A Unix socket path must stay under 108 bytes, so the control directory is short.
	control, err := os.MkdirTemp("", "wj")
	if err != nil {
		t.Fatal(err)
	}
	backend := &OpenSSH{
		Options:   Options{User: "deployer", Port: port, ConfigFiles: []string{config}, ControlPersist: 30},
		Processes: OSProcesses{}, ControlDir: control, MaxConcurrentStarts: 30,
		DNS: DNSRetry{Retries: 3, Sleep: time.Sleep},
	}
	defer backend.Close()
	waitForSSH(t, backend)

	var log bytes.Buffer
	result, err := backend.Run("127.0.0.1", NewCommand(shell.Words("echo", "hello from wunjo")), "id", NewFormatter(&log, false))
	if err != nil || result.ExitStatus != 0 || result.Stdout != "hello from wunjo\n" {
		t.Fatalf("result %+v err %v log %s", result, err, log.String())
	}
}

func startAgent(t *testing.T, dir, key string) {
	t.Helper()
	sock := filepath.Join(dir, "agent.sock")
	agent := exec.Command("ssh-agent", "-D", "-a", sock)
	if err := agent.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = agent.Process.Kill() })
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(sock); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Setenv("SSH_AUTH_SOCK", sock)
	run(t, "ssh-add", "-q", key)
}

func startSSHD(t *testing.T, publicKey string) string {
	t.Helper()
	id := strings.TrimSpace(run(t, "docker", "run", "-d", "--rm", "-p", "127.0.0.1::2222",
		"-e", "PUBLIC_KEY="+publicKey, "-e", "USER_NAME=deployer", "lscr.io/linuxserver/openssh-server:latest"))
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", id).Run() })
	mapping := strings.TrimSpace(run(t, "docker", "port", id, "2222/tcp"))
	return mapping[strings.LastIndex(mapping, ":")+1:]
}

func waitForSSH(t *testing.T, backend *OpenSSH) {
	t.Helper()
	quiet := NewFormatter(io.Discard, false)
	var last error
	for i := 0; i < 60; i++ {
		result, err := backend.Run("127.0.0.1", NewCommand(shell.Words("true")), "probe", quiet)
		if err == nil && result.ExitStatus == 0 {
			return
		}
		last = err
		time.Sleep(time.Second)
	}
	t.Fatalf("sshd did not accept the key within 60 seconds: %v", last)
}

func run(t *testing.T, name string, args ...string) string {
	t.Helper()
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return string(out)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
