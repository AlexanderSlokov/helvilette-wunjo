package configuration

// Ports Configuration#to_h from lib/kamal/configuration.rb at Kamal 2.12.0, which
// `kamal config` prints.

import "github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"

// ToH returns the summary of the configuration in Kamal's key order, without unset
// keys.
func (c *Configuration) ToH() (*yamlord.Ordered, error) {
	ssh, err := c.ssh.ToH()
	if err != nil {
		return nil, err
	}
	out := yamlord.NewOrdered()
	out.Set("roles", yamlord.Strings(c.RoleNames()...))
	out.Set("hosts", yamlord.Strings(c.AllHosts()...))
	if host := c.PrimaryHost(); host != "" {
		out.Set("primary_host", yamlord.NewString(host))
	}
	c.setImageFields(out)
	out.Set("volume_args", yamlord.Strings(c.VolumeArgs()...))
	out.Set("ssh_options", yamlord.NewMap(ssh))
	out.Set("sshkit", yamlord.NewMap(c.sshkit.ToH()))
	out.Set("builder", yamlord.NewMap(c.builder.ToH()))
	setIfPresent(out, "accessories", c.raw.At("accessories"))
	out.Set("logging", yamlord.Strings(c.LoggingArgs()...))
	return out, nil
}

func (c *Configuration) setImageFields(out *yamlord.Ordered) {
	out.Set("version", yamlord.NewString(c.Version()))
	out.Set("repository", yamlord.NewString(c.Repository()))
	out.Set("absolute_image", yamlord.NewString(c.AbsoluteImage()))
	out.Set("service_with_version", yamlord.NewString(c.ServiceWithVersion()))
}
