// Command wunjo deploys containers to the operator's servers over SSH. It runs the
// commands of Kamal 2.12.0, as ported in internal/kamal/.
package main

import "os"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	world := newSystem()
	defer world.close()
	return world.cli().Run(args)
}
