package cli

// Ports the upgrade command and the registry and network preparation of lib/kamal/cli/accessory.rb at Kamal 2.12.0.

import (
	"strings"
	"sync"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/commands"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/sshexec"
)

func (c *CLI) accessoryUpgrade(name string, opts Options) error {
	return c.confirming(opts, "This will restart all accessories", func() error {
		return c.modify(true, func() error {
			groups := [][]string{c.kamal.AccessoryHosts()}
			if opts.Bool("rolling") {
				groups = nil
				for _, host := range c.kamal.AccessoryHosts() {
					groups = append(groups, []string{host})
				}
			}
			for _, hosts := range groups {
				if err := c.upgradeAccessoryOn(name, hosts, opts); err != nil {
					return err
				}
			}
			return nil
		})
	})
}

func (c *CLI) upgradeAccessoryOn(name string, hosts []string, opts Options) error {
	list := strings.Join(hosts, ",")
	return c.kamal.WithSpecificHosts(hosts, func() error {
		c.say("Upgrading "+name+" accessories on "+list+"...", "magenta")
		if err := c.accessoryReboot(name, opts); err != nil {
			return err
		}
		c.say("Upgraded "+name+" accessories on "+list+"...", "magenta")
		return nil
	})
}

// supersededImageIDs reads each host's current accessory image before the
// container goes, so the old image can go once the new one is up.
func (c *CLI) supersededImageIDs(name string, opts Options) (map[string]string, error) {
	var mu sync.Mutex
	ids := map[string]string{}
	err := c.withAccessory(name, func(accessory *commands.Accessory, hosts []string) error {
		return c.on(opts, hosts, func(s *sshexec.Session) error {
			id, err := s.CaptureWithInfo(lenientCmd(accessory.CurrentImageID()))
			mu.Lock()
			ids[s.Host] = strings.TrimSpace(id)
			mu.Unlock()
			return err
		})
	})
	return ids, err
}

func (c *CLI) removeSupersededImages(name string, ids map[string]string, opts Options) error {
	return c.withAccessory(name, func(accessory *commands.Accessory, hosts []string) error {
		return c.on(opts, hosts, func(s *sshexec.Session) error {
			id := ids[s.Host]
			if id == "" {
				return nil
			}
			ok, err := s.ExecuteOK(cmd(accessory.RemoveImageID(id)))
			if err == nil && !ok {
				s.Log(sshexec.Info, "Kept "+name+" image "+truncate(id, 19)+" on "+s.Host+", docker declined to remove it")
			}
			return err
		})
	})
}

func truncate(text string, n int) string {
	if len(text) <= n {
		return text
	}
	return text[:n]
}

// accessoryPrepare logs in to the accessory's registry and creates the kamal
// network, which may already exist.
func (c *CLI) accessoryPrepare(name string, opts Options) error {
	return c.withAccessory(name, func(accessory *commands.Accessory, hosts []string) error {
		return c.on(opts, hosts, func(s *sshexec.Session) error {
			err := c.loginAndCreateNetwork(s, name)
			if err != nil && strings.Contains(err.Error(), "already exists") {
				return nil
			}
			return err
		})
	})
}

func (c *CLI) loginAndCreateNetwork(s *sshexec.Session, name string) error {
	login, err := c.kamal.Registry().Login(c.config().Accessory(name).Registry())
	if err != nil {
		return err
	}
	if login != nil {
		if err := s.Execute(cmd(login)); err != nil {
			return err
		}
	}
	return s.Execute(cmd(c.kamal.Docker().CreateNetwork()))
}
