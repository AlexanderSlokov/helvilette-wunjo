// Package commands builds the shell commands that Kamal runs on hosts and on the
// operator's machine. Each builder returns a shell.Line whose String() is the exact
// command that Kamal 2.12.0 builds for the same configuration.
//
// Ports lib/kamal/commands/base.rb at Kamal 2.12.0.
package commands

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/git"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/tags"
)

// Line is one command line.
type Line = shell.Line

// dockerHealthStatusFormat prints the health status, or the state without a health
// check.
const dockerHealthStatusFormat = "'{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}'"

// Runtime holds what the commands read besides the configuration. Tests fix every
// field, so the commands are repeatable.
type Runtime struct {
	// Now returns the current time, for tags and lock details.
	Now func() time.Time
	// Performer names who runs the command, for tags.
	Performer string
	// Git reads the local checkout, for clones and lock details.
	Git git.Git
	// RandomHex returns 2n random hex characters, as SecureRandom.hex(n) does.
	RandomHex func(n int) string
	// StdinTTY reports whether standard input is a terminal.
	StdinTTY bool
	// Files checks local files, such as the Dockerfile and hooks.
	Files configuration.FileSystem
	// WorkDir is the directory that relative local paths start from.
	WorkDir string
}

// Base holds the configuration and runtime that every command builder reads.
type Base struct {
	cfg *configuration.Configuration
	rt  Runtime
}

// NewBase returns the shared part of every builder.
func NewBase(cfg *configuration.Configuration, rt Runtime) Base { return Base{cfg: cfg, rt: rt} }

// Config returns the configuration.
func (b Base) Config() *configuration.Configuration { return b.cfg }

// RunOverSSH returns an ssh command that runs command on host with a terminal, as
// run_over_ssh does.
//
//	b.RunOverSSH(w("docker", "ps"), "1.1.1.1") // ssh -t root@1.1.1.1 -p 22 'docker ps'
func (b Base) RunOverSSH(command Line, host string) string {
	ssh := b.cfg.SSH()
	quoted := strings.ReplaceAll(command.String(), "'", `'\''`)
	return "ssh" + b.sshConfigArgs() + b.sshProxyArgs() + b.sshKeysArgs() +
		" -t " + ssh.User() + "@" + host + " -p " + ssh.Port() + " '" + quoted + "'"
}

// ContainerIDFor returns `docker container ls` filtered to the exact name.
func (b Base) ContainerIDFor(containerName string, onlyRunning bool) Line {
	return docker(w("container", "ls"), when(!onlyRunning, "--all"), w("--filter", "'name=^"+containerName+"$'", "--quiet"))
}

// MakeDirectoryFor returns mkdir -p of the directory that holds remoteFile.
func (b Base) MakeDirectoryFor(remoteFile string) Line {
	return makeDirectory(filepath.Dir(remoteFile))
}

// MakeDirectory returns mkdir -p path.
func (b Base) MakeDirectory(path string) Line { return makeDirectory(path) }

// RemoveDirectory returns rm -r path.
func (b Base) RemoveDirectory(path string) Line { return w("rm", "-r", path) }

// RemoveFile returns rm path.
func (b Base) RemoveFile(path string) Line { return w("rm", path) }

// EnsureDockerInstalled checks the local docker and buildx.
func (b Base) EnsureDockerInstalled() Line {
	return combine("&&", docker(w("--version")), docker(w("buildx", "version")))
}

func makeDirectory(path string) Line { return w("mkdir", "-p", path) }

func (b Base) sshConfigArgs() string {
	config := b.cfg.SSH().Config()
	if items := config.List(); items != nil {
		var out strings.Builder
		for _, file := range config.StringList() {
			out.WriteString(" -F " + file)
		}
		return out.String()
	}
	switch {
	case config.IsFalse():
		return " -F /dev/null"
	case config.IsTrue() || config.IsNull():
		return ""
	default:
		return " -F " + config.ToS()
	}
}

func (b Base) sshProxyArgs() string {
	proxy, ok := b.cfg.SSH().Proxy()
	switch {
	case !ok:
		return ""
	case proxy.Jump != "":
		return " -J " + proxy.Jump
	default:
		return " -o ProxyCommand='" + proxy.Command + "'"
	}
}

func (b Base) sshKeysArgs() string {
	var out strings.Builder
	for _, key := range b.cfg.SSH().Keys().StringList() {
		out.WriteString(" -i " + key)
	}
	if b.cfg.SSH().KeysOnly().Truthy() {
		out.WriteString(" -o IdentitiesOnly=yes")
	}
	return out.String()
}

func (b Base) dockerInteractiveArgs() string {
	if b.rt.StdinTTY {
		return "-it"
	}
	return "-i"
}

// tags returns the default tags of the deploy with extra details, as
// Kamal::Tags.from_config(config, **details) does.
func (b Base) tags(details ...tags.Tag) tags.Tags {
	d := tags.Deploy{Destination: b.cfg.Destination(), Version: b.cfg.Version(), Service: b.cfg.Service(), AbbreviatedVersion: b.cfg.AbbreviatedVersion()}
	return tags.Default(d, b.rt.Now(), b.rt.Performer).With(details...)
}

// w returns plain words, for example w("docker", "ps").
func w(texts ...string) Line { return shell.Words(texts...) }

// cat joins lines into one, as Ruby's [*a, *b] does. Nil lines add nothing.
func cat(lines ...Line) Line { return shell.Concat(lines...) }

// when returns the words when cond holds and nothing otherwise, as
// *("--all" if cond) does.
func when(cond bool, texts ...string) Line {
	if !cond {
		return nil
	}
	return w(texts...)
}

func docker(parts ...Line) Line { return cat(append([]Line{w("docker")}, parts...)...) }

func pack(parts ...Line) Line { return cat(append([]Line{w("pack")}, parts...)...) }

func grep(parts ...Line) Line { return cat(append([]Line{w("grep")}, parts...)...) }

// gitIn returns git [-C path] args.
func gitIn(path string, args ...string) Line {
	return cat(w("git"), when(path != "", "-C", path), w(args...))
}

// combine joins commands with by between them, skipping nil commands, as combine
// does. An empty but non-nil command still gets a separator, as in Ruby.
func combine(by string, commands ...Line) Line {
	var out Line
	for _, command := range commands {
		if command == nil {
			continue
		}
		if out != nil {
			out = append(out, shell.Plain(by))
		}
		out = append(append(Line{}, out...), command...)
	}
	return out
}

func chain(commands ...Line) Line { return combine(";", commands...) }

func pipe(commands ...Line) Line { return combine("|", commands...) }

func appendTo(commands ...Line) Line { return combine(">>", commands...) }

func writeTo(commands ...Line) Line { return combine(">", commands...) }

func anyOf(commands ...Line) Line { return combine("||", commands...) }

// substitute returns $(command).
func substitute(commands ...Line) string { return "$(" + cat(commands...).String() + ")" }

func xargs(command Line) Line { return cat(w("xargs"), command) }

// shellCommand returns sh -c '<command>', with single quotes escaped.
func shellCommand(command Line) Line {
	return w("sh", "-c", "'"+strings.ReplaceAll(command.String(), "'", `'\''`)+"'")
}

// wordsOf returns the text of each word.
func wordsOf(line Line) []string {
	out := make([]string, len(line))
	for i, word := range line {
		out[i] = word.String()
	}
	return out
}
