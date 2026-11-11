package cli

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/cliframe"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/commander"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/sshexec"
)

// Runtime holds everything the commands read or change outside the configuration.
// The binary wires real implementations; tests pass fakes.
type Runtime struct {
	Stdout    io.Writer
	Stderr    io.Writer
	Stdin     io.Reader
	StdoutTTY bool
	// Remote returns the backend that runs commands on hosts for a configuration.
	Remote func(cfg *configuration.Configuration) sshexec.Backend
	// Local runs commands on this machine.
	Local sshexec.Backend
	// Exec replaces the wunjo process with a shell command, as Ruby's exec does for
	// interactive ssh sessions.
	Exec func(command string) error
	// Forward opens an SSH remote port forward to each host until the returned stop
	// function runs, for a registry on localhost.
	Forward func(hosts []string, port int, ssh sshexec.Options) (stop func(), err error)
	Sleep   func(time.Duration)
	Now     func() time.Time
	// RandomHex returns 2n random hex characters.
	RandomHex func(n int) string
	// Files reads, writes, and checks local files.
	Files LocalFiles
	// Getenv reads the process environment.
	Getenv func(name string) (string, bool)
	// Adapters looks up the secrets adapters of `kamal secrets fetch`.
	Adapters SecretsAdapters
}

// LocalFiles is the local file system that init, error pages, and clones use.
type LocalFiles interface {
	configuration.FileSystem
	WriteFile(path string, content []byte, mode uint32) error
	MkdirAll(path string) error
	RemoveAll(path string) error
	Glob(pattern string) ([]string, error)
}

// CLI runs one wunjo command line.
type CLI struct {
	rt      Runtime
	kamal   *commander.Commander
	out     *sshexec.Formatter
	remote  sshexec.Backend
	invoked map[string]bool
	first   []string
	globals Options
	// inlineResult holds the output of an inline `kamal secrets` call.
	inlineResult string
	// invoke runs a command by its Thor name, such as "build:deliver". Tests replace
	// it to record invocations, as Kamal's tests expect calls to invoke.
	invoke func(name string, args []string, opts Options) error
}

// New returns a CLI that runs commands with kamal and rt.
func New(kamal *commander.Commander, rt Runtime) *CLI {
	c := &CLI{rt: rt, kamal: kamal, out: sshexec.NewFormatter(rt.Stdout, rt.StdoutTTY), invoked: map[string]bool{}}
	c.invoke = c.runInvocation
	return c
}

// RunInline runs `kamal secrets ...` args inside this process and returns what the
// command would print, as a secrets file that calls kamal secrets needs.
func (c *CLI) RunInline(args []string) (string, error) {
	inner := New(c.kamal, c.rt)
	inner.inlineResult = ""
	if err := inner.execute(args); err != nil {
		return "", err
	}
	return inner.inlineResult, nil
}

// Run parses args and runs the command. It returns the exit status and prints
// errors as bin/kamal does: "  ERROR (<class>): <message>" in red.
//
//	status := cli.New(kamal, rt).Run(os.Args[1:])
func (c *CLI) Run(args []string) (status int) {
	defer func() {
		if r := recover(); r != nil {
			status = c.reportPanic(r)
		}
	}()
	if err := c.execute(args); err != nil {
		c.reportError(err)
		return 1
	}
	return 0
}

// reportPanic reports the errors that Kamal raises from deep inside a command: a
// missing version and a configuration that fails to load. Any other panic is a bug.
func (c *CLI) reportPanic(r any) int {
	err, ok := r.(error)
	if !ok {
		panic(r)
	}
	var version *configuration.VersionError
	var load *commander.LoadError
	if !errors.As(err, &version) && !errors.As(err, &load) {
		panic(r)
	}
	c.reportError(reportable(err))
	return 1
}

func (c *CLI) reportError(err error) {
	var parse *cliframe.ParseError
	if errors.As(err, &parse) {
		fmt.Fprintln(c.rt.Stderr, parse.Message)
		return
	}
	var exec *sshexec.ExecuteError
	class := errorClass(err)
	if errors.As(err, &exec) {
		class = errorClass(exec.Cause)
	}
	fmt.Fprintf(c.rt.Stdout, "  \033[31mERROR (%s): %s\033[0m\n", class, err.Error())
}

// errorClass names the Ruby exception class that Kamal would raise for err.
func errorClass(err error) string {
	var classed interface{ RubyClass() string }
	if errors.As(err, &classed) {
		return classed.RubyClass()
	}
	var failed *sshexec.FailedError
	if errors.As(err, &failed) {
		return "SSHKit::Command::Failed"
	}
	if class, ok := errorClassOfConfig(err); ok {
		return class
	}
	if errors.Is(err, configuration.ErrDestinationRequired) {
		return "ArgumentError"
	}
	return "RuntimeError"
}

// Error is an error with the name of the Kamal exception class it ports, such as
// Kamal::Cli::LockError.
type Error struct {
	Class   string
	Message string
}

func (e *Error) Error() string { return e.Message }

// RubyClass returns the Kamal exception class.
func (e *Error) RubyClass() string { return e.Class }

func lockError(message string) error { return &Error{Class: "Kamal::Cli::LockError", Message: message} }
func bootError(message string) error { return &Error{Class: "Kamal::Cli::BootError", Message: message} }
func hookError(message string) error { return &Error{Class: "Kamal::Cli::HookError", Message: message} }
func argumentError(message string) error {
	return &Error{Class: "ArgumentError", Message: message}
}
func dependencyError(message string) error {
	return &Error{Class: "Kamal::Cli::DependencyError", Message: message}
}
