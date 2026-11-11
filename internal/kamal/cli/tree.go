package cli

// Ports the Thor command tree of lib/kamal/cli/*.rb at Kamal 2.12.0: each command
// keeps Kamal's name, arguments, description, and options.

import (
	"strconv"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/cliframe"
)

// flag describes one Thor option. A flag with a default appears in Options even
// when the user does not set it, as Thor's options hash does.
type flag struct {
	name, short, usage string
	kind               cliframe.FlagKind
	defaultValue       string
	hasDefault         bool
	repeatable         bool
}

func boolFlag(name, short, usage string) flag {
	return flag{name: name, short: short, usage: usage, kind: cliframe.BoolFlag, defaultValue: "false", hasDefault: true}
}

func optionalBool(name, short, usage string) flag {
	return flag{name: name, short: short, usage: usage, kind: cliframe.BoolFlag}
}

func stringFlag(name, short, usage string) flag {
	return flag{name: name, short: short, usage: usage, kind: cliframe.StringFlag}
}

func numberFlag(name, short, usage string) flag {
	return flag{name: name, short: short, usage: usage, kind: cliframe.IntFlag}
}

func withDefault(f flag, value string) flag {
	f.defaultValue, f.hasDefault = value, true
	return f
}

func repeatable(f flag) flag {
	f.repeatable, f.kind = true, cliframe.StringListFlag
	return f
}

// classOptions are Kamal::Cli::Base's class options, which every command accepts.
var classOptions = []flag{
	optionalBool("verbose", "v", "Detailed logging"),
	optionalBool("quiet", "q", "Minimal logging"),
	stringFlag("version", "", "Run commands against a specific app version"),
	optionalBool("primary", "p", "Run commands only on primary host instead of all"),
	stringFlag("hosts", "h", "Run commands on these hosts instead of all (separate by comma, supports wildcards with *)"),
	stringFlag("roles", "r", "Run commands on these roles instead of all (separate by comma, supports wildcards with *)"),
	withDefault(stringFlag("config_file", "c", "Path to config file"), "config/deploy.yml"),
	stringFlag("destination", "d", "Specify destination to be used for config file (staging -> deploy.staging.yml)"),
	boolFlag("skip_hooks", "H", "Don't run hooks"),
	boolFlag("lock_wait", "", "Wait for the deploy lock if it's already held instead of failing immediately"),
	withDefault(numberFlag("lock_wait_timeout", "", "Maximum seconds to wait for the deploy lock when --lock-wait is set"), "900"),
	withDefault(numberFlag("lock_wait_interval", "", "Seconds between deploy lock polls when --lock-wait is set"), "15"),
}

// spec is one Thor command.
type spec struct {
	name, args, desc string
	hidden           bool
	minArgs, maxArgs int
	flags            []flag
	run              func(args []string, opts Options) error
	children         []spec
	passUnknownFlags bool
}

func toFrameFlags(flags []flag) []cliframe.Flag {
	out := make([]cliframe.Flag, len(flags))
	for i, f := range flags {
		out[i] = cliframe.Flag{Name: dashed(f.name), Short: f.short, Usage: f.usage, Kind: f.kind, Default: frameDefault(f)}
	}
	return out
}

func frameDefault(f flag) string {
	if f.kind == cliframe.StringListFlag {
		return ""
	}
	return f.defaultValue
}

func dashed(name string) string { return strings.ReplaceAll(name, "_", "-") }

// optionsFrom reads the flags that the user set or that have a default.
func optionsFrom(inv *cliframe.Invocation, flags []flag) Options {
	opts := Options{}
	for _, f := range flags {
		name := dashed(f.name)
		if !inv.Changed(name) && !f.hasDefault {
			continue
		}
		opts[f.name] = flagText(inv, f, name)
	}
	return opts
}

func flagText(inv *cliframe.Invocation, f flag, name string) string {
	switch f.kind {
	case cliframe.BoolFlag:
		return strconv.FormatBool(inv.Bool(name))
	case cliframe.IntFlag:
		return strconv.Itoa(inv.Int(name))
	case cliframe.StringListFlag:
		return strings.Join(inv.Strings(name), "\n")
	default:
		return inv.String(name)
	}
}
