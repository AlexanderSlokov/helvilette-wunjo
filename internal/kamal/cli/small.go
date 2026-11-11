package cli

// Ports lib/kamal/cli/registry.rb, server.rb, lock.rb, prune.rb, and secrets.rb at
// Kamal 2.12.0.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/secrets/adapters"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/sshexec"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

func noArgs(fn func(Options) error) func([]string, Options) error {
	return func(_ []string, o Options) error { return fn(o) }
}

var registryFlags = []flag{
	boolFlag("skip_local", "L", "Skip local login"),
	boolFlag("skip_remote", "R", "Skip remote login"),
}

func (c *CLI) registrySpec() spec {
	return spec{name: "registry", desc: "Login and -out of the image registry", children: []spec{
		{name: "setup", desc: "Setup local registry or log in to remote registry locally and remotely", flags: registryFlags, run: noArgs(c.registrySetup)},
		{name: "remove", desc: "Remove local registry or log out of remote registry locally and remotely", flags: registryFlags, run: noArgs(c.registryRemove)},
		{name: "login", desc: "Log in to remote registry locally and remotely", flags: registryFlags, run: noArgs(c.registryLogin)},
		{name: "logout", desc: "Log out of remote registry locally and remotely", flags: registryFlags, run: noArgs(c.registryLogout)},
	}}
}

func (c *CLI) registrySetup(opts Options) error {
	if !opts.Bool("skip_local") {
		if err := c.ensureDockerInstalled(); err != nil {
			return err
		}
	}
	registry := c.kamal.Registry()
	if registry.Local() {
		return c.unlessSkipped(opts.Bool("skip_local"), func() error {
			return c.runLocally(nil, func(s *sshexec.Session) error { return s.Execute(cmd(registry.Setup(nil))) })
		})
	}
	if err := c.unlessSkipped(opts.Bool("skip_local"), func() error { return c.runLocally(nil, c.executeLogin) }); err != nil {
		return err
	}
	return c.unlessSkipped(opts.Bool("skip_remote"), func() error { return c.on(opts, c.kamal.Hosts(), c.executeLogin) })
}

func (c *CLI) unlessSkipped(skip bool, fn func() error) error {
	if skip {
		return nil
	}
	return fn()
}

func (c *CLI) registryRemove(opts Options) error {
	registry := c.kamal.Registry()
	if registry.Local() {
		return c.unlessSkipped(opts.Bool("skip_local"), func() error {
			return c.runLocally(nil, func(s *sshexec.Session) error { return s.Execute(lenientCmd(registry.Remove())) })
		})
	}
	logout := func(s *sshexec.Session) error { return s.Execute(cmd(registry.Logout(nil))) }
	if err := c.unlessSkipped(opts.Bool("skip_local"), func() error { return c.runLocally(nil, logout) }); err != nil {
		return err
	}
	return c.unlessSkipped(opts.Bool("skip_remote"), func() error { return c.on(opts, c.kamal.Hosts(), logout) })
}

func (c *CLI) registryLogin(opts Options) error {
	if c.kamal.Registry().Local() {
		return errors.New("Cannot use login command with a local registry. Use `kamal registry setup` instead.")
	}
	return c.registrySetup(opts)
}

func (c *CLI) registryLogout(opts Options) error {
	if c.kamal.Registry().Local() {
		return errors.New("Cannot use logout command with a local registry. Use `kamal registry remove` instead.")
	}
	return c.registryRemove(opts)
}

func (c *CLI) serverSpec() spec {
	return spec{name: "server", desc: "Bootstrap servers with curl and Docker", children: []spec{
		{name: "exec", desc: "Run a custom command on the server (use --help to show options)", maxArgs: anyArgs,
			flags: []flag{boolFlag("interactive", "i", "Run the command interactively (use for console/bash)"), boolFlag("raw", "", "Output raw, unmodified stdout")}, run: c.serverExec},
		{name: "bootstrap", desc: "Set up Docker to run Kamal apps", run: noArgs(c.serverBootstrap)},
	}}
}

func (c *CLI) serverExec(args []string, opts Options) error {
	if opts.Bool("raw") && opts.Bool("interactive") {
		return argumentError("Raw is not compatible with interactive")
	}
	return c.withRawOutput(opts.Bool("raw"), func() error {
		if err := c.preConnectIfRequired(opts); err != nil {
			return err
		}
		command := utils.JoinCommands(args)
		if opts.Bool("interactive") {
			host := c.kamal.PrimaryHost()
			c.say("Running '"+command+"' on "+host+" interactively...", "magenta")
			return c.rt.Exec(c.kamal.Server().RunOverSSH(words(command), host))
		}
		hosts := c.kamal.Hosts()
		c.say("Running '"+command+"' on "+strings.Join(hosts, ", ")+"...", "magenta")
		return c.on(opts, hosts, func(s *sshexec.Session) error {
			if err := s.Execute(debugCmd(c.kamal.Auditor().Record("Executed cmd '" + command + "' on " + s.Host))); err != nil {
				return err
			}
			return c.captureExecOutput(s, opts, words(command))
		})
	})
}

func (c *CLI) serverBootstrap(opts Options) error {
	return c.modify(true, func() error {
		var mu sync.Mutex
		var missing []string
		err := c.on(opts, c.kamal.Hosts(), func(s *sshexec.Session) error {
			ok, err := c.installDocker(s)
			if err == nil && !ok {
				mu.Lock()
				missing = append(missing, s.Host)
				mu.Unlock()
			}
			return err
		})
		if err != nil {
			return err
		}
		if len(missing) > 0 {
			return errors.New("Docker is not installed on " + strings.Join(missing, ", ") + " and can't be automatically installed without having root access and either `wget` or `curl`. Install Docker manually: https://docs.docker.com/engine/install/")
		}
		return c.runHook(opts, "docker-setup", false)
	})
}

// installDocker installs Docker when it is missing and the user may, and reports
// false when Docker is missing and cannot be installed.
func (c *CLI) installDocker(s *sshexec.Session) (bool, error) {
	docker := c.kamal.Docker()
	if installed, err := s.ExecuteOK(cmd(docker.Installed())); err != nil || installed {
		return true, err
	}
	superuser, err := s.ExecuteOK(cmd(docker.Superuser()))
	if err != nil || !superuser {
		return false, err
	}
	s.Log(sshexec.Info, "Missing Docker on "+s.Host+". Installing…")
	if err := s.Execute(cmd(docker.Install())); err != nil {
		return true, err
	}
	return true, c.joinDockerGroup(s)
}

func (c *CLI) joinDockerGroup(s *sshexec.Session) error {
	docker := c.kamal.Docker()
	for _, check := range []shell.Line{docker.Root(), docker.InDockerGroup()} {
		if ok, err := s.ExecuteOK(cmd(check)); err != nil || ok {
			return err
		}
	}
	if err := s.Execute(cmd(docker.AddToDockerGroup())); err != nil {
		return err
	}
	if err := s.Execute(cmd(docker.RefreshSession())); err != nil {
		s.Log(sshexec.Info, "Session refreshed due to group change.")
	}
	return nil
}

func (c *CLI) lockSpec() spec {
	return spec{name: "lock", desc: "Manage the deploy lock", children: []spec{
		{name: "status", desc: "Report lock status", run: noArgs(c.lockStatus)},
		{name: "acquire", desc: "Acquire the deploy lock", flags: []flag{stringFlag("message", "m", "A lock message")}, run: noArgs(c.requireMessage(c.lockAcquire))},
		{name: "release", desc: "Release the deploy lock", run: noArgs(c.lockRelease)},
	}}
}

func (c *CLI) requireMessage(fn func(Options) error) func(Options) error {
	return func(opts Options) error {
		if opts.String("message") == "" {
			c.puts("No value provided for required options '--message'")
			return nil
		}
		return fn(opts)
	}
}

func (c *CLI) pruneSpec() spec {
	return spec{name: "prune", desc: "Prune old application images and containers", children: []spec{
		{name: "all", desc: "Prune unused images and stopped containers", run: noArgs(c.pruneAll)},
		{name: "images", desc: "Prune unused images", run: noArgs(c.pruneImages)},
		{name: "containers", desc: "Prune all stopped containers, except the last n (default 5)", flags: []flag{numberFlag("retain", "", "Number of containers to retain")}, run: noArgs(c.pruneContainers)},
	}}
}

func (c *CLI) pruneAll(opts Options) error {
	return c.modify(true, func() error {
		if err := c.pruneContainers(opts); err != nil {
			return err
		}
		return c.pruneImages(opts)
	})
}

func (c *CLI) pruneImages(opts Options) error {
	return c.modify(true, func() error {
		return c.on(opts, c.kamal.Hosts(), func(s *sshexec.Session) error {
			prune := c.kamal.Prune()
			return executeAll(s, debugCmd(c.kamal.Auditor().Record("Pruned images")), cmd(prune.DanglingImages()), cmd(prune.TaggedImages()))
		})
	})
}

func (c *CLI) pruneContainers(opts Options) error {
	retain, ok := opts.Int("retain")
	if !ok {
		retain = c.config().RetainContainers()
	}
	if retain < 1 {
		return errors.New("retain must be at least 1")
	}
	return c.modify(true, func() error {
		return c.on(opts, c.kamal.Hosts(), func(s *sshexec.Session) error {
			return executeAll(s, debugCmd(c.kamal.Auditor().Record("Pruned containers")), cmd(c.kamal.Prune().AppContainers(retain)))
		})
	})
}

// SecretsAdapter fetches secrets from a vault with the vendor's command-line tool,
// in the order of the vendor's output.
type SecretsAdapter interface {
	RequiresAccount() bool
	Fetch(secrets []string, account, from string) ([]adapters.Secret, error)
}

// SecretsAdapters looks up an adapter by name, such as "1password".
type SecretsAdapters func(name string) (SecretsAdapter, error)

func (c *CLI) secretsSpec() spec {
	inline := optionalBool("inline", "", "")
	return spec{name: "secrets", desc: "Helpers for extracting secrets", children: []spec{
		{name: "fetch", args: "[SECRETS...]", desc: "Fetch secrets from a vault", maxArgs: anyArgs, flags: []flag{
			stringFlag("adapter", "a", "Which vault adapter to use"), stringFlag("account", "", "The account identifier or username"),
			stringFlag("from", "", "A vault or folder to fetch the secrets from"), inline}, run: c.secretsFetch},
		{name: "extract", desc: "Extract a single secret from the results of a fetch call", minArgs: 2, maxArgs: 2, flags: []flag{inline}, run: c.secretsExtract},
		{name: "print", desc: "Print the secrets (for debugging)", run: noArgs(c.secretsPrint)},
	}}
}

func (c *CLI) secretsFetch(args []string, opts Options) error {
	if opts.String("adapter") == "" {
		c.puts("No value provided for required options '--adapter'")
		return nil
	}
	adapter, err := c.rt.Adapters(opts.String("adapter"))
	if err != nil {
		return err
	}
	if adapter.RequiresAccount() && strings.TrimSpace(opts.String("account")) == "" {
		c.puts("No value provided for required options '--account'")
		return nil
	}
	results, err := adapter.Fetch(args, opts.String("account"), opts.String("from"))
	if err != nil {
		return err
	}
	encoded := secretsJSON(results)
	if opts.Bool("inline") {
		encoded = shell.Escape(encoded)
	}
	return c.returnOrPuts(encoded, opts.Bool("inline"))
}

// secretsJSON encodes results in their order, as Ruby's JSON.dump writes a Hash:
// without HTML escapes, so "<" and "&" stay as they are.
func secretsJSON(results []adapters.Secret) string {
	parts := make([]string, len(results))
	for i, secret := range results {
		parts[i] = rubyJSONString(secret.Name) + ":" + rubyJSONString(secret.Value)
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func rubyJSONString(text string) string {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(text)
	return strings.TrimSuffix(out.String(), "\n")
}

func (c *CLI) secretsExtract(args []string, opts Options) error {
	var parsed map[string]string
	if err := json.Unmarshal([]byte(args[1]), &parsed); err != nil {
		return err
	}
	value, ok := parsed[args[0]]
	if !ok {
		value, ok = suffixMatch(parsed, args[0])
	}
	if !ok {
		return fmt.Errorf("Could not find secret %s", args[0])
	}
	return c.returnOrPuts(value, opts.Bool("inline"))
}

func suffixMatch(parsed map[string]string, name string) (string, bool) {
	keys := make([]string, 0, len(parsed))
	for key := range parsed {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if strings.HasSuffix(key, "/"+name) {
			return parsed[key], true
		}
	}
	return "", false
}

func (c *CLI) secretsPrint(_ Options) error {
	all, err := c.config().Secrets().All()
	if err != nil {
		return err
	}
	all.Each(func(key string, value *yamlord.Value) { c.puts(key + "=" + value.ToS()) })
	return nil
}

// returnOrPuts prints value, or hands it back for an inline `kamal secrets` call
// inside a secrets file.
func (c *CLI) returnOrPuts(value string, inline bool) error {
	if inline {
		c.inlineResult = value
		return nil
	}
	c.puts(value)
	return nil
}
