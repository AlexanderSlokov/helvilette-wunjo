package sshexec

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Options are the SSH settings of a deploy, from the ssh section of deploy.yml.
type Options struct {
	User         string
	Port         string
	ProxyJump    string
	ProxyCommand string
	Keys         []string
	KeysOnly     bool
	KeyData      []string
	// ConfigFiles replaces ~/.ssh/config when set. IgnoreConfig reads none.
	ConfigFiles  []string
	IgnoreConfig bool
	ForwardAgent *bool
	LogLevel     string
	// ControlPersist keeps each connection open for this many idle seconds, as
	// sshkit pool_idle_timeout does.
	ControlPersist int64
}

// OpenSSH runs commands with the operator's ssh client. Each host gets one master
// connection that later commands reuse, as SSHKit's connection pool does.
type OpenSSH struct {
	Options    Options
	Processes  ProcessRunner
	ControlDir string
	// MaxConcurrentStarts limits how many hosts open a connection at once.
	MaxConcurrentStarts int
	DNS                 DNSRetry

	once     sync.Once
	starts   chan struct{}
	mu       sync.Mutex
	warm     map[string]bool
	extraArg []string
	setupErr error
}

// Run logs cmd and runs it on host, retrying DNS failures.
func (o *OpenSSH) Run(host string, cmd Command, id string, out *Formatter) (Result, error) {
	out.commandStart(cmd, host, "", id)
	var result Result
	err := o.DNS.Do(host, out, func() error {
		var err error
		result, err = o.runOnce(host, cmd.ToCommand(), nil, id, out)
		return err
	})
	return result, err
}

func (o *OpenSSH) runOnce(host, command string, stdin io.Reader, id string, out *Formatter) (Result, error) {
	args, err := o.args(host, command)
	if err != nil {
		return Result{}, err
	}
	release := o.limitStart(host)
	result, err := runLogged(o.Processes, Process{Name: "ssh", Args: args, Stdin: stdin}, id, out)
	release(err == nil && result.ExitStatus != 255)
	if err != nil {
		return result, err
	}
	return result, connectionError(host, result)
}

// connectionError turns exit status 255, which ssh uses for its own failures, into
// an error, so a failed connection never reads as a failed command.
func connectionError(host string, result Result) error {
	if result.ExitStatus != 255 {
		return nil
	}
	return &ConnectionError{Host: host, Message: strings.TrimSpace(result.Stderr)}
}

// ConnectionError reports that ssh could not reach a host.
type ConnectionError struct {
	Host    string
	Message string
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("could not connect to %s: %s", e.Host, e.Message)
}

// Upload writes content to remote with cat, then sets its mode, as an SCP upload.
func (o *OpenSSH) Upload(host string, content Upload, remote string, out *Formatter) error {
	body, command, err := uploadPlan(content, remote)
	if err != nil {
		return err
	}
	err = o.DNS.Do(host, out, func() error {
		result, err := o.runOnce(host, command, bytes.NewReader(body.Bytes()), "", NewFormatter(io.Discard, false))
		if err == nil && result.ExitStatus != 0 {
			err = &FailedError{Name: "cat", ExitStatus: result.ExitStatus, Stdout: result.Stdout, Stderr: result.Stderr}
		}
		return err
	})
	if err == nil {
		out.Log(Info, "Uploading "+remote+" 100.0%")
	}
	return err
}

// LogsExit is true.
func (o *OpenSSH) LogsExit() bool { return true }

// AlwaysSucceeds is false.
func (o *OpenSSH) AlwaysSucceeds() bool { return false }

// Close ends the master connections and removes the control directory.
func (o *OpenSSH) Close() {
	o.mu.Lock()
	hosts := make([]string, 0, len(o.warm))
	for host := range o.warm {
		hosts = append(hosts, host)
	}
	o.mu.Unlock()
	for _, host := range hosts {
		args := append(o.baseArgs(), "-O", "exit", host)
		_, _ = o.Processes.RunProcess(Process{Name: "ssh", Args: args, Stdout: io.Discard, Stderr: io.Discard})
	}
	_ = os.RemoveAll(o.ControlDir)
}

// limitStart holds a start slot until the first connection to host finishes, so at
// most MaxConcurrentStarts connections open at once. Commands on a warm host skip
// the limit. The returned function marks the host warm when the connection worked.
func (o *OpenSSH) limitStart(host string) func(ok bool) {
	o.mu.Lock()
	warm := o.warm[host]
	o.mu.Unlock()
	if warm || o.MaxConcurrentStarts <= 0 {
		return func(bool) {}
	}
	o.once.Do(func() { o.starts = make(chan struct{}, o.MaxConcurrentStarts) })
	o.starts <- struct{}{}
	return func(ok bool) {
		if ok {
			o.mu.Lock()
			o.warm[host] = true
			o.mu.Unlock()
		}
		<-o.starts
	}
}

func (o *OpenSSH) args(host, command string) ([]string, error) {
	if err := o.setup(); err != nil {
		return nil, err
	}
	return append(o.baseArgs(), host, command), nil
}

// setup creates the control directory and writes key data and config includes once.
func (o *OpenSSH) setup() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.warm != nil {
		return o.setupErr
	}
	o.warm = map[string]bool{}
	if err := os.MkdirAll(o.ControlDir, 0o700); err != nil {
		o.setupErr = err
		return err
	}
	o.extraArg, o.setupErr = o.fileArgs()
	return o.setupErr
}

func (o *OpenSSH) fileArgs() ([]string, error) {
	var args []string
	for i, key := range o.Options.KeyData {
		path := filepath.Join(o.ControlDir, "key-"+strconv.Itoa(i))
		if err := os.WriteFile(path, []byte(strings.TrimSpace(key)+"\n"), 0o600); err != nil {
			return nil, err
		}
		args = append(args, "-i", path)
	}
	config, err := o.configArgs()
	return append(args, config...), err
}

// configArgs returns -F for the config files. OpenSSH reads one -F file, so several
// files go into a generated file that includes each of them.
func (o *OpenSSH) configArgs() ([]string, error) {
	switch {
	case o.Options.IgnoreConfig:
		return []string{"-F", "/dev/null"}, nil
	case len(o.Options.ConfigFiles) == 1:
		return []string{"-F", o.Options.ConfigFiles[0]}, nil
	case len(o.Options.ConfigFiles) > 1:
		return o.includeConfig()
	default:
		return nil, nil
	}
}

func (o *OpenSSH) includeConfig() ([]string, error) {
	var lines strings.Builder
	for _, file := range o.Options.ConfigFiles {
		lines.WriteString("Include " + file + "\n")
	}
	path := filepath.Join(o.ControlDir, "config")
	return []string{"-F", path}, os.WriteFile(path, []byte(lines.String()), 0o600)
}

// baseArgs maps the deploy file's SSH settings to ssh flags. StrictHostKeyChecking
// accept-new matches net-ssh's default, accept_new_or_local_tunnel.
func (o *OpenSSH) baseArgs() []string {
	opts := o.Options
	args := []string{
		"-o", "ControlMaster=auto", "-o", "ControlPath=" + filepath.Join(o.ControlDir, "%C"),
		"-o", "ControlPersist=" + strconv.FormatInt(max(opts.ControlPersist, 1), 10),
		"-o", "ServerAliveInterval=30", "-o", "StrictHostKeyChecking=accept-new", "-o", "LogLevel=" + sshLogLevel(opts.LogLevel),
	}
	args = append(args, optionalArgs(opts)...)
	return append(args, o.extraArg...)
}

func optionalArgs(opts Options) []string {
	var args []string
	add := func(cond bool, extra ...string) {
		if cond {
			args = append(args, extra...)
		}
	}
	add(opts.User != "", "-l", opts.User)
	add(opts.Port != "", "-p", opts.Port)
	add(opts.ProxyJump != "", "-J", opts.ProxyJump)
	add(opts.ProxyCommand != "", "-o", "ProxyCommand="+opts.ProxyCommand)
	for _, key := range opts.Keys {
		args = append(args, "-i", key)
	}
	add(opts.KeysOnly, "-o", "IdentitiesOnly=yes")
	add(opts.ForwardAgent != nil && *opts.ForwardAgent, "-A")
	add(opts.ForwardAgent != nil && !*opts.ForwardAgent, "-a")
	return args
}

// sshLogLevel maps Kamal's net-ssh log levels to OpenSSH's LogLevel.
func sshLogLevel(level string) string {
	switch strings.ToLower(level) {
	case "debug":
		return "DEBUG"
	case "info":
		return "INFO"
	case "warn":
		return "VERBOSE"
	default:
		return "ERROR"
	}
}

// uploadPlan returns what to send and the remote command that writes it.
func uploadPlan(content Upload, remote string) (*bytes.Buffer, string, error) {
	if content.Tree != nil {
		return treePlan(content, remote)
	}
	body, err := uploadBody(content)
	command := "cat > " + shellQuote(remote)
	if content.Mode != "" {
		command += " && chmod " + content.Mode + " " + shellQuote(remote)
	}
	return body, command, err
}

// treePlan sends a tar stream that unpacks into <remote>/<base>.
func treePlan(content Upload, remote string) (*bytes.Buffer, string, error) {
	target := shellQuote(remote + "/" + filepath.Base(content.LocalPath))
	body, err := tarTree(content.Tree)
	command := "mkdir -p " + target + " && tar -xf - -C " + target
	if content.Mode != "" {
		command += " && chmod " + content.Mode + " " + target
	}
	return body, command, err
}

func tarTree(files map[string][]byte) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(files[name]))}); err != nil {
			return nil, err
		}
		if _, err := tw.Write(files[name]); err != nil {
			return nil, err
		}
	}
	return &buf, tw.Close()
}

func uploadBody(content Upload) (*bytes.Buffer, error) {
	if content.InMemory {
		return bytes.NewBuffer(content.Content), nil
	}
	data, err := os.ReadFile(content.LocalPath)
	return bytes.NewBuffer(data), err
}

func shellQuote(path string) string { return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'" }

func statusError(what string, status int, err error) error {
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("%s exited with status %d", what, status)
	}
	return nil
}
