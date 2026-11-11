package cli

// Ports lib/kamal/cli/accessory.rb at Kamal 2.12.0.

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/commands"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/sshexec"
)

func (c *CLI) accessorySpec() spec {
	one := func(fn func(string, Options) error) func([]string, Options) error {
		return func(args []string, o Options) error { return fn(args[0], o) }
	}
	named := func(name, desc string, hidden bool, fn func(string, Options) error, flags ...flag) spec {
		return spec{name: name, args: "[NAME]", desc: desc, hidden: hidden, minArgs: 1, maxArgs: 1, flags: flags, run: one(fn)}
	}
	return spec{name: "accessory", desc: "Manage accessories (db/redis/search)", children: []spec{
		named("boot", "Boot new accessory service on host (use NAME=all to boot all accessories)", false, func(n string, o Options) error { return c.accessoryBoot(n, o, true) }),
		named("upload", "Upload accessory files to host", true, c.accessoryUpload),
		named("directories", "Create accessory directories on host", true, c.accessoryDirectories),
		named("reboot", "Reboot existing accessory on host (stop container, remove container, start new container; use NAME=all to boot all accessories)", false, c.accessoryReboot),
		named("start", "Start existing accessory container on host", false, c.accessoryStart),
		named("stop", "Stop existing accessory container on host", false, c.accessoryStop),
		named("restart", "Restart existing accessory container on host", false, c.accessoryRestart),
		named("details", "Show details about accessory on host (use NAME=all to show all accessories)", false, c.accessoryDetails),
		{name: "exec", args: "[NAME] [CMD...]", desc: "Execute a custom command on servers within the accessory container (use --help to show options)", minArgs: 1, maxArgs: anyArgs,
			flags: []flag{appExecFlags[0], appExecFlags[1], appExecFlags[4]}, run: c.accessoryExec},
		named("logs", "Show log lines from accessory on host (use --help to show options)", false, c.accessoryLogs, logFlags...),
		named("pull_image", "Pull accessory image on host", true, c.accessoryPullImage),
		named("remove", "Remove accessory container, image and data directory from host (use NAME=all to remove all accessories)", false, c.accessoryRemove, confirmedFlag),
		named("remove_container", "Remove accessory container from host", true, c.accessoryRemoveContainer),
		named("remove_image", "Remove accessory image from host", true, c.accessoryRemoveImage),
		named("remove_service_directory", "Remove accessory directory used for uploaded files and data directories from host", true, c.accessoryRemoveServiceDirectory),
		{name: "upgrade", desc: "Upgrade accessories from Kamal 1.x to 2.0 (restart them in 'kamal' network)", minArgs: 1, maxArgs: 1,
			flags: []flag{boolFlag("rolling", "", "Upgrade one host at a time"), confirmedFlag}, run: one(c.accessoryUpgrade)},
	}}
}

// withAccessory runs fn with the accessory and its selected hosts, or reports a
// missing accessory.
func (c *CLI) withAccessory(name string, fn func(*commands.Accessory, []string) error) error {
	if c.config().Accessory(name) == nil {
		return c.errorOnMissingAccessory(name)
	}
	accessory := c.kamal.Accessory(name)
	hosts := intersect(c.kamal.AccessoryHosts(), c.config().Accessory(name).Hosts())
	return fn(accessory, hosts)
}

func (c *CLI) errorOnMissingAccessory(name string) error {
	message := "No accessory by the name of '" + name + "'"
	if names := c.kamal.AccessoryNames(); len(names) > 0 {
		message += " (options: " + toSentence(names) + ")"
	}
	fmt.Fprintln(c.rt.Stderr, message)
	return nil
}

// toSentence joins names as ActiveSupport's to_sentence: "a, b, and c".
func toSentence(names []string) string {
	switch len(names) {
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	default:
		return strings.Join(names[:len(names)-1], ", ") + ", and " + names[len(names)-1]
	}
}

func intersect(a, b []string) []string {
	var out []string
	for _, item := range a {
		if containsString(b, item) && !containsString(out, item) {
			out = append(out, item)
		}
	}
	return out
}

func (c *CLI) eachAccessory(name string, fn func(string) error) error {
	if name != "all" {
		return fn(name)
	}
	for _, accessory := range c.kamal.AccessoryNames() {
		if err := fn(accessory); err != nil {
			return err
		}
	}
	return nil
}

func (c *CLI) accessoryBoot(name string, opts Options, prepare bool) error {
	return c.modify(true, func() error {
		if name == "all" {
			return c.eachAccessory(name, func(n string) error { return c.accessoryBoot(n, opts, true) })
		}
		if prepare {
			if err := c.accessoryPrepare(name, opts); err != nil {
				return err
			}
		}
		return c.withAccessory(name, func(accessory *commands.Accessory, hosts []string) error {
			return c.bootAccessoryOn(name, accessory, hosts, opts)
		})
	})
}

func (c *CLI) bootAccessoryOn(name string, accessory *commands.Accessory, hosts []string, opts Options) error {
	booted, err := c.bootedHosts(accessory, hosts, opts)
	if err != nil {
		return err
	}
	if len(booted) > 0 {
		c.say("Skipping booting `"+name+"` on "+strings.Join(booted, ", ")+", a container already exists", "yellow")
		hosts = subtractHosts(hosts, booted)
	}
	if err := c.accessoryDirectories(name, opts); err != nil {
		return err
	}
	if err := c.accessoryUpload(name, opts); err != nil {
		return err
	}
	return c.on(opts, hosts, func(s *sshexec.Session) error { return c.runAccessory(s, name, accessory) })
}

func (c *CLI) bootedHosts(accessory *commands.Accessory, hosts []string, opts Options) ([]string, error) {
	var mu sync.Mutex
	var booted []string
	err := c.on(opts, hosts, func(s *sshexec.Session) error {
		out, err := s.CaptureWithInfo(cmd(accessory.Info(true, true)))
		if strings.TrimSpace(out) != "" {
			mu.Lock()
			booted = append(booted, s.Host)
			mu.Unlock()
		}
		return err
	})
	sort.Strings(booted)
	return booted, err
}

func subtractHosts(hosts, remove []string) []string {
	var out []string
	for _, host := range hosts {
		if !containsString(remove, host) {
			out = append(out, host)
		}
	}
	return out
}

func (c *CLI) runAccessory(s *sshexec.Session, name string, accessory *commands.Accessory) error {
	if err := s.Execute(debugCmd(c.kamal.Auditor().Record("Booted " + name + " accessory"))); err != nil {
		return err
	}
	if err := s.Execute(cmd(accessory.EnsureEnvDirectory())); err != nil {
		return err
	}
	if err := c.uploadAccessorySecrets(s, name); err != nil {
		return err
	}
	run, err := accessory.Run(s.Host)
	if err != nil {
		return err
	}
	if err := s.Execute(cmd(run)); err != nil {
		return err
	}
	return c.deployAccessoryToProxy(s, name, accessory)
}

func (c *CLI) uploadAccessorySecrets(s *sshexec.Session, name string) error {
	acc := c.config().Accessory(name)
	secrets, err := acc.SecretsFile()
	if err != nil {
		return err
	}
	return s.Upload(sshexec.Upload{Content: []byte(secrets), InMemory: true, Mode: "0600"}, acc.SecretsPath())
}

func (c *CLI) deployAccessoryToProxy(s *sshexec.Session, name string, accessory *commands.Accessory) error {
	acc := c.config().Accessory(name)
	if !acc.RunningProxy() {
		return nil
	}
	target, err := s.CaptureWithInfo(cmd(accessory.ContainerIDFor(acc.ServiceName(), true)))
	if err != nil {
		return err
	}
	return s.Execute(cmd(accessory.Deploy(strings.TrimSpace(target))))
}

func (c *CLI) accessoryUpload(name string, opts Options) error {
	return c.modify(true, func() error {
		return c.withAccessory(name, func(accessory *commands.Accessory, hosts []string) error {
			files, err := c.config().Accessory(name).Files()
			if err != nil {
				return err
			}
			return c.on(opts, hosts, func(s *sshexec.Session) error { return c.uploadAccessoryFiles(s, accessory, files) })
		})
	})
}

func (c *CLI) uploadAccessoryFiles(s *sshexec.Session, accessory *commands.Accessory, files []configuration.AccessoryPath) error {
	for _, file := range files {
		if err := accessory.EnsureLocalFilePresent(file); err != nil {
			return err
		}
		if err := s.Execute(cmd(accessory.MakeDirectoryFor(file.HostPath))); err != nil {
			return err
		}
		upload := sshexec.Upload{LocalPath: file.Local}
		if file.Rendered {
			upload = sshexec.Upload{Content: []byte(file.Content), InMemory: true}
		}
		if err := s.Upload(upload, file.HostPath); err != nil {
			return err
		}
		if err := c.chmodChown(s, file.Mode, file.Owner, file.HostPath); err != nil {
			return err
		}
	}
	return nil
}

func (c *CLI) chmodChown(s *sshexec.Session, mode, owner, path string) error {
	if mode != "" {
		if err := s.Execute(cmd(words("chmod", mode, path))); err != nil {
			return err
		}
	}
	if owner != "" {
		return s.Execute(cmd(words("chown", owner, path)))
	}
	return nil
}

func (c *CLI) accessoryDirectories(name string, opts Options) error {
	return c.modify(true, func() error {
		return c.withAccessory(name, func(accessory *commands.Accessory, hosts []string) error {
			return c.on(opts, hosts, func(s *sshexec.Session) error {
				for _, dir := range c.config().Accessory(name).Directories() {
					if err := s.Execute(cmd(accessory.MakeDirectory(dir.Local))); err != nil {
						return err
					}
					if err := c.chmodChown(s, dir.Mode, dir.Owner, dir.Local); err != nil {
						return err
					}
				}
				return nil
			})
		})
	})
}

func (c *CLI) accessoryReboot(name string, opts Options) error {
	return c.modify(true, func() error {
		if name == "all" {
			return c.eachAccessory(name, func(n string) error { return c.accessoryReboot(n, opts) })
		}
		if err := c.accessoryPrepare(name, opts); err != nil {
			return err
		}
		if err := c.accessoryPullImage(name, opts); err != nil {
			return err
		}
		superseded, err := c.supersededImageIDs(name, opts)
		if err != nil {
			return err
		}
		steps := []func() error{
			func() error { return c.accessoryStop(name, opts) }, func() error { return c.accessoryRemoveContainer(name, opts) },
			func() error { return c.accessoryBoot(name, opts, false) }, func() error { return c.removeSupersededImages(name, superseded, opts) },
		}
		return runSteps(steps)
	})
}

func (c *CLI) accessoryStart(name string, opts Options) error {
	return c.modify(true, func() error {
		return c.withAccessory(name, func(accessory *commands.Accessory, hosts []string) error {
			return c.on(opts, hosts, func(s *sshexec.Session) error {
				if err := s.Execute(debugCmd(c.kamal.Auditor().Record("Started " + name + " accessory"))); err != nil {
					return err
				}
				if err := s.Execute(cmd(accessory.Start())); err != nil {
					return err
				}
				return c.deployAccessoryToProxy(s, name, accessory)
			})
		})
	})
}

func (c *CLI) accessoryStop(name string, opts Options) error {
	return c.modify(true, func() error {
		return c.withAccessory(name, func(accessory *commands.Accessory, hosts []string) error {
			return c.on(opts, hosts, func(s *sshexec.Session) error {
				if err := s.Execute(debugCmd(c.kamal.Auditor().Record("Stopped " + name + " accessory"))); err != nil {
					return err
				}
				if err := s.Execute(lenientCmd(accessory.Stop())); err != nil {
					return err
				}
				return c.removeAccessoryFromProxy(s, name, accessory)
			})
		})
	})
}

func (c *CLI) removeAccessoryFromProxy(s *sshexec.Session, name string, accessory *commands.Accessory) error {
	acc := c.config().Accessory(name)
	if !acc.RunningProxy() {
		return nil
	}
	if _, err := s.CaptureWithInfo(cmd(accessory.ContainerIDFor(acc.ServiceName(), true))); err != nil {
		return err
	}
	return s.Execute(cmd(accessory.Remove()))
}

func (c *CLI) accessoryRestart(name string, opts Options) error {
	return c.modify(true, func() error {
		if err := c.accessoryStop(name, opts); err != nil {
			return err
		}
		return c.accessoryStart(name, opts)
	})
}

func (c *CLI) accessoryDetails(name string, opts Options) error {
	return c.eachAccessory(name, func(n string) error {
		return c.withAccessory(n, func(accessory *commands.Accessory, hosts []string) error {
			return c.on(opts, hosts, func(s *sshexec.Session) error {
				output, err := s.CaptureWithInfo(cmd(accessory.Info(false, false)))
				c.putsByHost(s.Host, output, "Accessory "+n, opts.Bool("quiet"), false)
				return err
			})
		})
	})
}

func (c *CLI) accessoryPullImage(name string, opts Options) error {
	return c.modify(true, func() error {
		return c.withAccessory(name, func(accessory *commands.Accessory, hosts []string) error {
			return c.on(opts, hosts, func(s *sshexec.Session) error {
				if err := s.Execute(debugCmd(c.kamal.Auditor().Record("Pull " + name + " accessory image"))); err != nil {
					return err
				}
				return s.Execute(cmd(accessory.PullImage()))
			})
		})
	})
}

func (c *CLI) accessoryRemove(name string, opts Options) error {
	question := "This will remove all containers, images and data directories for " + name + ". Are you sure?"
	return c.confirming(opts, question, func() error {
		return c.modify(true, func() error {
			return c.eachAccessory(name, func(n string) error { return c.removeAccessory(n, opts) })
		})
	})
}

func (c *CLI) removeAccessory(name string, opts Options) error {
	return runSteps([]func() error{
		func() error { return c.accessoryStop(name, opts) }, func() error { return c.accessoryRemoveContainer(name, opts) },
		func() error { return c.accessoryRemoveImage(name, opts) }, func() error { return c.accessoryRemoveServiceDirectory(name, opts) },
	})
}

func (c *CLI) accessoryOnHosts(name, audit string, opts Options, line func(*commands.Accessory) commands.Line) error {
	return c.modify(true, func() error {
		return c.withAccessory(name, func(accessory *commands.Accessory, hosts []string) error {
			return c.on(opts, hosts, func(s *sshexec.Session) error {
				if audit != "" {
					if err := s.Execute(debugCmd(c.kamal.Auditor().Record(audit))); err != nil {
						return err
					}
				}
				return s.Execute(cmd(line(accessory)))
			})
		})
	})
}

func (c *CLI) accessoryRemoveContainer(name string, opts Options) error {
	return c.accessoryOnHosts(name, "Remove "+name+" accessory container", opts, (*commands.Accessory).RemoveContainer)
}

func (c *CLI) accessoryRemoveImage(name string, opts Options) error {
	return c.accessoryOnHosts(name, "Removed "+name+" accessory image", opts, (*commands.Accessory).RemoveImage)
}

func (c *CLI) accessoryRemoveServiceDirectory(name string, opts Options) error {
	return c.accessoryOnHosts(name, "", opts, (*commands.Accessory).RemoveServiceDirectory)
}
