// Command wunjo deploys containers to the operator's servers over SSH.
package main

import (
	"fmt"
	"os"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/cliframe"
)

// version is the Wunjo release. The ported behavior matches Kamal 2.12.0.
const version = "0.1.0"

func main() {
	if err := cliframe.Execute(rootCommand(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func rootCommand() *cliframe.Command {
	return &cliframe.Command{
		Name:     "wunjo",
		Short:    "Deploy containers to your own servers over SSH",
		Children: []*cliframe.Command{versionCommand()},
	}
}

func versionCommand() *cliframe.Command {
	return &cliframe.Command{
		Name:  "version",
		Short: "Show Wunjo version",
		Run: func(inv *cliframe.Invocation) error {
			_, err := fmt.Fprintln(inv.Stdout, version)
			return err
		},
	}
}
