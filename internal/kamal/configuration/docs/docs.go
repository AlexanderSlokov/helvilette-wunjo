// Package docs embeds Kamal's configuration documents. Kamal's validator compares the
// keys of a deploy file against the example keys in these files, and `kamal docs`
// prints them, so the port ships them unchanged.
//
// Ports lib/kamal/configuration/docs/ at Kamal 2.12.0.
package docs

import (
	"embed"
	"fmt"
)

//go:embed *.yml
var files embed.FS

// Read returns the document for one configuration section, for example Read("proxy").
func Read(section string) ([]byte, error) {
	content, err := files.ReadFile(section + ".yml")
	if err != nil {
		return nil, fmt.Errorf("no configuration document for section %q: want one of accessory, alias, boot, builder, configuration, env, logging, output, proxy, registry, role, servers, ssh, sshkit", section)
	}
	return content, nil
}
