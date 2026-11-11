package cli

// Ports Thor#tree and Thor#build_command_tree from thor 1.4.0, which Kamal 2.12.0
// inherits as `kamal tree`.

import (
	"sort"
	"strings"
)

const treeDescription = "Print a tree of all available commands"

// printCommandTree prints the main commands, then the commands of each
// subcommand group, as `kamal tree` does:
//
//	kamal:cli:main
//	  ├─ accessory (Manage accessories (db/redis/search))
func (c *CLI) printCommandTree() {
	main := c.specs()
	c.printTreeGroup("kamal:cli:main", "", main)
	for _, group := range main {
		if len(group.children) > 0 {
			c.printTreeGroup("kamal:cli:"+group.name, "  ", group.children)
		}
	}
}

func (c *CLI) printTreeGroup(namespace, indent string, specs []spec) {
	c.say(indent+namespace, "blue")
	visible := treeVisible(specs)
	for i, s := range visible {
		icon := "├─"
		if i == len(visible)-1 {
			icon = "└─"
		}
		c.sayPart(indent+"  "+icon+" ", "")
		c.sayPart(s.name, "green")
		c.say(" ("+strings.SplitN(s.desc, "\n", 2)[0]+")", "")
	}
}

// treeVisible returns the commands that Thor lists, by name. Thor leaves out
// hidden commands and help. tree itself is a command of the Thor base class,
// so Kamal's class lists it nowhere.
func treeVisible(specs []spec) []spec {
	var visible []spec
	for _, s := range specs {
		if !s.hidden && s.name != "help" && s.name != "tree" {
			visible = append(visible, s)
		}
	}
	sort.Slice(visible, func(i, j int) bool { return visible[i].name < visible[j].name })
	return visible
}
