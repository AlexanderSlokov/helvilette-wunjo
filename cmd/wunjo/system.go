package main

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/cli"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/commander"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/commands"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/git"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/output"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/secrets"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/secrets/adapters"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/sshexec"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/tags"
)

// system wires the real operating system into the CLI: the process environment,
// local files, local commands, and the OpenSSH client.
type system struct {
	workDir    string
	controlDir string
	shell      localshell.System
	ssh        *sshexec.OpenSSH
	inline     *cli.CLI
}

func newSystem() *system {
	wd, _ := os.Getwd()
	control, _ := os.MkdirTemp("", "wj")
	return &system{workDir: wd, controlDir: control, shell: localshell.System{Stderr: os.Stderr}}
}

func (s *system) close() {
	if s.ssh != nil {
		s.ssh.Close()
	}
	_ = os.RemoveAll(s.controlDir)
}

func (s *system) cli() *cli.CLI {
	kamal := commander.New(s.environment(), s.commandRuntime(), os.Getenv("KAMAL_LOCK") == "true")
	c := cli.New(kamal, s.runtime())
	s.inline = c
	return c
}

func (s *system) environment() configuration.Environment {
	g := git.Git{Shell: s.shell}
	return configuration.Environment{
		Lookup: os.LookupEnv, Shell: s.shell, Git: g, Files: configuration.OSFileSystem{}, SecretFiles: secrets.OSFiles{},
		Substitution: secrets.Substitution{Shell: s.shell, Lookup: os.LookupEnv, InlineSecrets: s.inlineSecrets},
		WorkDir:      s.workDir, TempDir: os.TempDir(), RandomHex: func() string { return randomHex(8) }, Stderr: os.Stderr,
	}
}

// inlineSecrets runs `kamal secrets ...` inside this process, as Kamal does for a
// secrets file that calls it.
func (s *system) inlineSecrets(args []string) (string, error) {
	return s.inline.RunInline(args)
}

func (s *system) commandRuntime() commands.Runtime {
	return commands.Runtime{Now: time.Now, Performer: tags.Performer(s.shell), Git: git.Git{Shell: s.shell},
		RandomHex: randomHex, StdinTTY: isTerminal(os.Stdin), Files: configuration.OSFileSystem{}, WorkDir: s.workDir}
}

func (s *system) runtime() cli.Runtime {
	return cli.Runtime{
		Stdout: os.Stdout, Stderr: os.Stderr, Stdin: os.Stdin, StdoutTTY: isTerminal(os.Stdout),
		Remote: s.remote, Local: sshexec.Local{Processes: sshexec.OSProcesses{}, User: currentUser()}, Exec: execShell, Forward: s.forward,
		Sleep: time.Sleep, Now: time.Now, RandomHex: randomHex, Files: osFiles{},
		Getenv: func(name string) (string, bool) {
			if name == "PWD" {
				return s.workDir, true
			}
			return os.LookupEnv(name)
		},
		Adapters:    s.secretsAdapter,
		OutputFiles: output.OSFiles{},
		OTelPoster:  output.NewHTTPPoster(),
	}
}

func (s *system) remote(cfg *configuration.Configuration) sshexec.Backend {
	retries, _ := cfg.Sshkit().DNSRetries()
	s.ssh = &sshexec.OpenSSH{Options: cli.SSHOptions(cfg), Processes: sshexec.OSProcesses{}, ControlDir: s.controlDir,
		MaxConcurrentStarts: int(cfg.Sshkit().MaxConcurrentStarts()), DNS: sshexec.DNSRetry{Retries: int(retries), Sleep: time.Sleep}}
	return s.ssh
}

// forward opens `ssh -N -R port:localhost:port` to each host until stop runs.
func (s *system) forward(hosts []string, port int, opts sshexec.Options) (func(), error) {
	var procs []*exec.Cmd
	stop := func() {
		for _, p := range procs {
			_ = p.Process.Kill()
			_ = p.Wait()
		}
	}
	spec := strconv.Itoa(port) + ":localhost:" + strconv.Itoa(port)
	for _, host := range hosts {
		args := append(forwardArgs(opts), "-N", "-o", "ExitOnForwardFailure=yes", "-R", "127.0.0.1:"+spec, host)
		p := exec.Command("ssh", args...)
		p.Stderr = os.Stderr
		if err := p.Start(); err != nil {
			stop()
			return nil, err
		}
		procs = append(procs, p)
	}
	time.Sleep(time.Second)
	return stop, nil
}

func forwardArgs(opts sshexec.Options) []string {
	var args []string
	if opts.User != "" {
		args = append(args, "-l", opts.User)
	}
	if opts.Port != "" {
		args = append(args, "-p", opts.Port)
	}
	return args
}

// execShell replaces wunjo with the shell command, as Ruby's exec does for
// interactive sessions.
func execShell(command string) error {
	return syscall.Exec("/bin/sh", []string{"sh", "-c", command}, os.Environ())
}

// currentUser returns the login that SSHKit shows for local commands.
func currentUser() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return os.Getenv("USER")
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// secretsAdapter looks up a vault adapter that calls the vendor tools on this machine.
func (s *system) secretsAdapter(name string) (cli.SecretsAdapter, error) {
	adapter, err := adapters.Lookup(name, adapters.Deps{Shell: s.shell, Lookup: os.LookupEnv})
	if err != nil {
		return nil, err
	}
	return adapter, nil
}

// osFiles is the local file system of init, error pages, and clones.
type osFiles struct{ configuration.OSFileSystem }

func (osFiles) WriteFile(path string, content []byte, mode uint32) error {
	return os.WriteFile(path, content, os.FileMode(mode))
}

func (osFiles) MkdirAll(path string) error { return os.MkdirAll(path, 0o755) }

func (osFiles) RemoveAll(path string) error { return os.RemoveAll(path) }

func (osFiles) Glob(pattern string) ([]string, error) { return filepath.Glob(pattern) }
