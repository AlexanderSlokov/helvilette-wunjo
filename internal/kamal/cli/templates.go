package cli

import "embed"

// templates holds the files of `kamal init`, copied from
// lib/kamal/cli/templates/ at Kamal 2.12.0. One comment in deploy.yml changes: its
// example ERB used Ruby that Wunjo's ERB subset rejects, and ERB renders comments
// too, so it now reads ENV["RUBY_VERSION"].
//
//go:embed templates
var templates embed.FS
