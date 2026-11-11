package cli

// Ports Kamal::Cli::Base#initialize_commander, Thor's invoke, and
// lib/kamal/cli/alias/command.rb at Kamal 2.12.0.

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/cliframe"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/commander"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/kamalerr"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
)

// specs returns the commands of Kamal::Cli::Main, with the subcommands.
func (c *CLI) specs() []spec {
	return append(c.mainSpecs(), c.accessorySpec(), c.appSpec(), c.buildSpec(), c.lockSpec(),
		c.proxySpec(), c.pruneSpec(), c.registrySpec(), c.secretsSpec(), c.serverSpec())
}

func (c *CLI) execute(args []string) error {
	if alias, ok := c.aliasFor(args); ok {
		return c.runAlias(alias, args)
	}
	root := &cliframe.Command{Name: "wunjo", Short: "Deploy containers to your own servers over SSH", GlobalFlags: toFrameFlags(classOptions)}
	for _, s := range c.specs() {
		root.Children = append(root.Children, c.frame(s, nil))
	}
	return cliframe.Execute(root, args, c.rt.Stdout, c.rt.Stderr)
}

func (c *CLI) frame(s spec, parent []string) *cliframe.Command {
	path := append(append([]string{}, parent...), s.name)
	cmd := &cliframe.Command{Name: s.name, ArgsUsage: s.args, Short: s.desc, Hidden: s.hidden,
		MinArgs: s.minArgs, MaxArgs: s.maxArgs, Flags: toFrameFlags(s.flags)}
	if s.run != nil {
		cmd.Run = func(inv *cliframe.Invocation) error {
			return c.start(path, inv.Args, optionsFrom(inv, append(append([]flag{}, classOptions...), s.flags...)), s.run)
		}
	}
	for _, child := range s.children {
		cmd.Children = append(cmd.Children, c.frame(child, path))
	}
	return cmd
}

// start runs the command that the user typed, after configuring the commander.
func (c *CLI) start(path, args []string, opts Options, run func([]string, Options) error) error {
	c.first = path
	c.invoked[strings.Join(path, ":")] = true
	c.globals = opts
	if !c.kamal.Configured() {
		if err := c.initializeCommander(opts); err != nil {
			return err
		}
	}
	return run(args, opts)
}

// initializeCommander applies the class options, as initialize_commander does.
func (c *CLI) initializeCommander(opts Options) error {
	if opts.Bool("verbose") {
		c.kamal.Verbosity = verbosityLevels["verbose"]
	}
	if opts.Bool("quiet") {
		c.kamal.Verbosity = verbosityLevels["quiet"]
	}
	c.kamal.Configure(commander.ConfigOptions{ConfigFile: c.absolute(opts.String("config_file")),
		Destination: opts.String("destination"), Version: opts.String("version")})
	if err := c.applySelection(opts); err != nil {
		return err
	}
	c.kamal.LockWait = opts.Bool("lock_wait")
	timeout, _ := opts.Int("lock_wait_timeout")
	interval, _ := opts.Int("lock_wait_interval")
	c.kamal.LockWaitTimeout, c.kamal.LockWaitInterval = int(timeout), int(interval)
	return nil
}

func (c *CLI) applySelection(opts Options) error {
	if hosts := opts.String("hosts"); hosts != "" {
		if err := c.kamal.SetSpecificHosts(strings.Split(hosts, ",")); err != nil {
			return argumentError(err.Error())
		}
	}
	if roles := opts.String("roles"); roles != "" {
		if err := c.kamal.SetSpecificRoles(strings.Split(roles, ",")); err != nil {
			return argumentError(err.Error())
		}
	}
	if opts.Bool("primary") {
		c.kamal.SpecificPrimary()
	}
	return nil
}

func (c *CLI) absolute(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	wd, _ := c.rt.Getenv("PWD")
	return filepath.Join(wd, path)
}

// runInvocation runs a command by its Thor name once per run, as Thor's invoke
// does: a second invoke of the same command does nothing.
func (c *CLI) runInvocation(name string, args []string, opts Options) error {
	if c.invoked[name] {
		return nil
	}
	c.invoked[name] = true
	run := c.handler(name)
	if run == nil {
		return fmt.Errorf("cli: no command %q", name)
	}
	return run(args, opts)
}

// resetInvocation lets invoke run the commands of group again, as
// reset_invocation does.
func (c *CLI) resetInvocation(group string) {
	for name := range c.invoked {
		if strings.HasPrefix(name, group+":") {
			delete(c.invoked, name)
		}
	}
}

func (c *CLI) handler(name string) func([]string, Options) error {
	for _, s := range c.specs() {
		if found := findHandler(s, "", name); found != nil {
			return found
		}
	}
	return nil
}

func findHandler(s spec, prefix, name string) func([]string, Options) error {
	full := strings.TrimPrefix(prefix+":"+s.name, ":")
	if full == name {
		return s.run
	}
	for _, child := range s.children {
		if found := findHandler(child, full, name); found != nil {
			return found
		}
	}
	return nil
}

// aliasFor returns the alias command for args when the first word is no command
// and the deploy file defines it as an alias.
func (c *CLI) aliasFor(args []string) (string, bool) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") || c.isCommand(args[0]) {
		return "", false
	}
	opts := aliasOptions(args[1:])
	c.kamal.Configure(commander.ConfigOptions{ConfigFile: c.absolute(opts.String("config_file")), Destination: opts.String("destination")})
	command, ok, err := c.kamal.ResolveAlias(args[0])
	if err != nil || !ok {
		c.kamal.Reset()
		return "", false
	}
	return command, true
}

func (c *CLI) isCommand(name string) bool {
	for _, s := range c.specs() {
		if s.name == name {
			return true
		}
	}
	return name == "help" || name == "completion"
}

// aliasOptions reads -c and -d from the arguments after an alias name.
func aliasOptions(args []string) Options {
	opts := Options{"config_file": "config/deploy.yml"}
	for i := 0; i+1 < len(args); i++ {
		switch args[i] {
		case "-c", "--config-file", "--config_file":
			opts["config_file"] = args[i+1]
		case "-d", "--destination":
			opts["destination"] = args[i+1]
		}
	}
	return opts
}

func (c *CLI) runAlias(command string, args []string) error {
	words, err := shell.Split(command)
	if err != nil {
		return err
	}
	c.kamal.Reset()
	c.invoked = map[string]bool{}
	return c.execute(append(words, args[1:]...))
}

// globalOptions returns the options of the command the user typed.
func (c *CLI) globalOptions() Options { return c.globals }

// reportable turns a recovered load error into an error to print.
func reportable(err error) error {
	var load *commander.LoadError
	if errors.As(err, &load) {
		return load.Err
	}
	return err
}

// errorClassOfConfig names configuration errors as Kamal's exception classes.
func errorClassOfConfig(err error) (string, bool) {
	var configErr *kamalerr.ConfigurationError
	if errors.As(err, &configErr) {
		return "Kamal::ConfigurationError", true
	}
	return "", false
}
