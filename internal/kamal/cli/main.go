package cli

// Ports lib/kamal/cli/main.rb at Kamal 2.12.0.

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration/docs"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/sshexec"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/tags"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// Version is the Wunjo release that `wunjo version` prints.
const Version = "0.1.0"

var deployFlags = []flag{
	boolFlag("skip_push", "P", "Skip image build and push"),
	boolFlag("no_cache", "", "Build without using Docker's build cache"),
}

var confirmedFlag = boolFlag("confirmed", "y", "Proceed without confirmation question")

func (c *CLI) mainSpecs() []spec {
	return []spec{
		{name: "setup", desc: "Setup all accessories, push the env, and deploy app to servers", flags: deployFlags, run: c.setup},
		{name: "deploy", desc: "Deploy app to servers", flags: deployFlags, run: func(_ []string, o Options) error { return c.deploy(o, false) }},
		{name: "redeploy", desc: "Deploy app to servers without bootstrapping servers, starting kamal-proxy and pruning", flags: deployFlags, run: c.redeploy},
		{name: "rollback", args: "[VERSION]", desc: "Rollback app to VERSION", minArgs: 1, maxArgs: 1, run: c.rollback},
		{name: "details", desc: "Show details about all containers", run: c.details},
		{name: "audit", desc: "Show audit log from servers", run: c.audit},
		{name: "config", desc: "Show combined config (including secrets!)", run: c.showConfig},
		{name: "docs", args: "[SECTION]", desc: "Show Kamal configuration documentation", maxArgs: 1, run: c.docs},
		{name: "init", desc: "Create config stub in config/deploy.yml and secrets stub in .kamal", flags: []flag{boolFlag("bundle", "", "Add Kamal to the Gemfile and create a bin/kamal binstub")}, run: c.initProject},
		{name: "remove", desc: "Remove kamal-proxy, app, accessories, and registry session from servers", flags: []flag{confirmedFlag}, run: c.removeAll},
		{name: "upgrade", desc: "Upgrade from Kamal 1.x to 2.0", flags: []flag{confirmedFlag, boolFlag("rolling", "", "Upgrade one host at a time")}, run: c.upgrade},
		{name: "version", desc: "Show Kamal version", run: func([]string, Options) error { c.puts(Version); return nil }},
	}
}

func (c *CLI) setup(_ []string, opts Options) error {
	_, err := c.printRuntime(func() error {
		return c.modify(true, func() error {
			invokeOptions := c.deployOptions(opts)
			c.say("Ensure Docker is installed...", "magenta")
			if err := c.invoke("server:bootstrap", nil, invokeOptions); err != nil {
				return err
			}
			return c.deploy(opts, true)
		})
	})
	return err
}

func (c *CLI) deploy(opts Options, bootAccessories bool) error {
	runtime, err := c.printRuntime(func() error {
		invokeOptions := c.deployOptions(opts)
		if err := c.buildOrPull(opts, invokeOptions); err != nil {
			return err
		}
		return c.modify(true, func() error { return c.deploySteps(opts, invokeOptions, bootAccessories) })
	})
	if err != nil {
		return err
	}
	return c.runHook(opts, "post-deploy", true, runtimeTag(runtime))
}

func (c *CLI) buildOrPull(opts, invokeOptions Options) error {
	if opts.Bool("skip_push") {
		c.say("Pull app image...", "magenta")
		return c.invoke("build:pull", nil, invokeOptions)
	}
	c.say("Build and push app image...", "magenta")
	return c.invoke("build:deliver", nil, invokeOptions)
}

func (c *CLI) deploySteps(opts, invokeOptions Options, bootAccessories bool) error {
	if err := c.runHook(opts, "pre-deploy", true); err != nil {
		return err
	}
	c.say("Ensure kamal-proxy is running...", "magenta")
	if err := c.invoke("proxy:boot", nil, invokeOptions); err != nil {
		return err
	}
	if bootAccessories {
		if err := c.invoke("accessory:boot", []string{"all"}, invokeOptions); err != nil {
			return err
		}
	}
	if err := c.staleAndBoot(invokeOptions); err != nil {
		return err
	}
	c.say("Prune old containers and images...", "magenta")
	return c.invoke("prune:all", nil, invokeOptions)
}

func (c *CLI) staleAndBoot(invokeOptions Options) error {
	c.say("Detect stale containers...", "magenta")
	if err := c.invoke("app:stale_containers", nil, invokeOptions.With("stop", "true")); err != nil {
		return err
	}
	return c.invoke("app:boot", nil, invokeOptions)
}

func runtimeTag(runtime time.Duration) tags.Tag {
	return tags.Tag{Name: "runtime", Value: strconv.FormatInt(int64(runtime.Seconds()+0.5), 10)}
}

func (c *CLI) redeploy(_ []string, opts Options) error {
	runtime, err := c.printRuntime(func() error {
		invokeOptions := c.deployOptions(opts)
		if err := c.buildOrPull(opts, invokeOptions); err != nil {
			return err
		}
		return c.modify(true, func() error {
			if err := c.runHook(opts, "pre-deploy", true); err != nil {
				return err
			}
			return c.staleAndBoot(invokeOptions)
		})
	})
	if err != nil {
		return err
	}
	return c.runHook(opts, "post-deploy", true, runtimeTag(runtime))
}

func (c *CLI) rollback(args []string, opts Options) error {
	version, rolledBack := args[0], false
	runtime, err := c.printRuntime(func() error {
		return c.modify(true, func() error {
			invokeOptions := c.deployOptions(opts)
			c.config().SetVersion(version)
			available, err := c.containerAvailable(opts, version)
			if err != nil || !available {
				if err == nil {
					c.say("The app version '"+version+"' is not available as a container (use 'kamal app containers' for available versions)", "red")
				}
				return err
			}
			if err := c.runHook(opts, "pre-deploy", true); err != nil {
				return err
			}
			rolledBack = true
			return c.invoke("app:boot", nil, invokeOptions.With("version", version))
		})
	})
	if err != nil || !rolledBack {
		return err
	}
	return c.runHook(opts, "post-deploy", true, runtimeTag(runtime))
}

var containerNotFound = regexp.MustCompile(`Container not found`)

func (c *CLI) containerAvailable(opts Options, version string) (bool, error) {
	err := c.on(opts, c.kamal.AppHosts(), func(s *sshexec.Session) error {
		for _, role := range c.kamal.RolesOn(s.Host) {
			id, err := s.CaptureWithInfo(cmd(c.kamal.App(role, s.Host).ContainerIDForVersion(version, false)))
			if err != nil {
				return err
			}
			if strings.TrimSpace(id) == "" {
				return errors.New("Container not found")
			}
		}
		return nil
	})
	if err != nil && containerNotFound.MatchString(err.Error()) {
		c.say("Error looking for container version "+version+": "+err.Error(), "")
		return false, nil
	}
	return err == nil, err
}

// deployOptions passes the options on to the invoked commands, with the version
// first, without skip_push, and without no_cache unless it is set.
func (c *CLI) deployOptions(opts Options) Options {
	base := opts.Without("skip_push")
	if !base.Bool("no_cache") {
		base = base.Without("no_cache")
	}
	return Options{"version": c.config().Version()}.Merge(base)
}

func (c *CLI) details(_ []string, opts Options) error {
	if err := c.invoke("proxy:details", nil, opts); err != nil {
		return err
	}
	if err := c.invoke("app:details", nil, opts); err != nil {
		return err
	}
	return c.invoke("accessory:details", []string{"all"}, opts)
}

func (c *CLI) audit(_ []string, opts Options) error {
	return c.on(opts, c.kamal.Hosts(), func(s *sshexec.Session) error {
		output, err := s.CaptureWithInfo(cmd(c.kamal.Auditor().Reveal()))
		c.putsByHost(s.Host, output, "App", opts.Bool("quiet"), false)
		return err
	})
}

func (c *CLI) showConfig(_ []string, _ Options) error {
	h, err := c.config().ToH()
	if err != nil {
		return err
	}
	c.puts(yamlord.DumpWithSymbolKeys(yamlord.NewMap(h)))
	return nil
}

// docs prints a configuration document, as `kamal docs [SECTION]` does.
func (c *CLI) docs(args []string, _ Options) error {
	section := "configuration"
	if len(args) > 0 {
		section = strings.ToLower(args[0])
	}
	content, err := docs.Read(section)
	if err != nil {
		c.puts("No documentation found for " + args[0])
		return nil
	}
	c.puts(string(content))
	return nil
}

// initProject writes config/deploy.yml, .kamal/secrets, and the sample hooks when
// they are missing, as `kamal init` does.
func (c *CLI) initProject(_ []string, opts Options) error {
	steps := []struct{ target, source, created, exists string }{
		{"config/deploy.yml", "templates/deploy.yml", "Created configuration file in config/deploy.yml", "Config file already exists in config/deploy.yml (remove first to create a new one)"},
		{".kamal/secrets", "templates/secrets", "Created .kamal/secrets file", ""},
	}
	for _, step := range steps {
		if err := c.copyTemplate(step.target, step.source, step.created, step.exists); err != nil {
			return err
		}
	}
	if err := c.copySampleHooks(); err != nil {
		return err
	}
	if opts.Bool("bundle") {
		return c.bundleKamal()
	}
	return nil
}

func (c *CLI) copyTemplate(target, source, created, exists string) error {
	path := c.absolute(target)
	if c.rt.Files.Exists(path) {
		if exists != "" {
			c.puts(exists)
		}
		return nil
	}
	content, err := templates.ReadFile(source)
	if err != nil {
		return err
	}
	if err := c.rt.Files.MkdirAll(filepath.Dir(path)); err != nil {
		return err
	}
	if err := c.rt.Files.WriteFile(path, content, 0o644); err != nil {
		return err
	}
	c.puts(created)
	return nil
}

func (c *CLI) copySampleHooks() error {
	dir := c.absolute(".kamal/hooks")
	if c.rt.Files.Exists(dir) {
		return nil
	}
	if err := c.rt.Files.MkdirAll(dir); err != nil {
		return err
	}
	samples, err := fs.ReadDir(templates, "templates/sample_hooks")
	if err != nil {
		return err
	}
	for _, sample := range samples {
		content, _ := templates.ReadFile(path.Join("templates/sample_hooks", sample.Name()))
		if err := c.rt.Files.WriteFile(filepath.Join(dir, sample.Name()), content, 0o755); err != nil {
			return err
		}
	}
	c.puts("Created sample hooks in .kamal/hooks")
	return nil
}

func (c *CLI) bundleKamal() error {
	if c.rt.Files.Exists(c.absolute("bin/kamal")) {
		c.puts("Binstub already exists in bin/kamal (remove first to create a new one)")
		return nil
	}
	c.puts("Adding Kamal to Gemfile and bundle...")
	err := c.runLocally(nil, func(s *sshexec.Session) error {
		if err := s.Execute(cmd(words("bundle", "add", "kamal"))); err != nil {
			return err
		}
		return s.Execute(cmd(words("bundle", "binstubs", "kamal")))
	})
	if err == nil {
		c.puts("Created binstub file in bin/kamal")
	}
	return err
}

func (c *CLI) removeAll(_ []string, opts Options) error {
	return c.confirming(opts, "This will remove all containers and images. Are you sure?", func() error {
		return c.modify(true, func() error {
			withoutConfirm := opts.Without("confirmed")
			steps := []struct {
				name string
				args []string
				opts Options
			}{
				{"app:remove", nil, withoutConfirm}, {"proxy:remove", nil, withoutConfirm},
				{"accessory:remove", []string{"all"}, opts}, {"registry:remove", nil, withoutConfirm.With("skip_local", "true")},
			}
			for _, step := range steps {
				if err := c.invoke(step.name, step.args, step.opts); err != nil {
					return err
				}
			}
			return nil
		})
	})
}

func (c *CLI) upgrade(_ []string, opts Options) error {
	return c.confirming(opts, "This will replace Traefik with kamal-proxy and restart all accessories", func() error {
		return c.modify(true, func() error {
			if opts.Bool("rolling") {
				return c.rollingUpgrade(opts)
			}
			c.say("Upgrading all hosts...", "magenta")
			confirmed := opts.With("confirmed", "true")
			if err := c.invoke("proxy:upgrade", nil, confirmed); err != nil {
				return err
			}
			if err := c.invoke("accessory:upgrade", []string{"all"}, confirmed); err != nil {
				return err
			}
			c.say("Upgraded all hosts", "magenta")
			return nil
		})
	})
}

func (c *CLI) rollingUpgrade(opts Options) error {
	step := opts.With("confirmed", "true").With("rolling", "false")
	for _, host := range c.kamal.Hosts() {
		err := c.kamal.WithSpecificHosts([]string{host}, func() error { return c.upgradeHost(host, step) })
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *CLI) upgradeHost(host string, step Options) error {
	c.say("Upgrading "+host+"...", "magenta")
	if containsString(c.kamal.AppHosts(), host) {
		if err := c.invoke("proxy:upgrade", nil, step); err != nil {
			return err
		}
		c.resetInvocation("proxy")
	}
	if containsString(c.kamal.AccessoryHosts(), host) {
		if err := c.invoke("accessory:upgrade", []string{"all"}, step); err != nil {
			return err
		}
		c.resetInvocation("accessory")
	}
	c.say("Upgraded "+host, "magenta")
	return nil
}

// putsByHost prints output under a host header, as puts_by_host does.
func (c *CLI) putsByHost(host, output, kind string, quiet, raw bool) {
	sshexec.PutsByHost(func(s string) { fmt.Fprint(c.rt.Stdout, s) }, host, output, kind, quiet, raw)
}
