// Package cli implements the wunjo commands, as Kamal's Thor command-line interface
// does: deploy, rollback, app, accessory, build, proxy, registry, server, lock,
// prune, and secrets.
//
// Ports lib/kamal/cli.rb and lib/kamal/cli/ at Kamal 2.12.0.
package cli

import (
	"sort"
	"strconv"
	"strings"
)

// Options are the parsed options of one command, as Thor's options hash. Values keep
// the text the user typed; booleans are "true" or "false".
type Options map[string]string

// Bool reports whether the boolean option is set to true.
func (o Options) Bool(name string) bool { return o[name] == "true" }

// String returns the text of an option, or "".
func (o Options) String(name string) string { return o[name] }

// Has reports whether the option has a value.
func (o Options) Has(name string) bool {
	_, ok := o[name]
	return ok
}

// Int returns the integer of a numeric option and whether it is set.
func (o Options) Int(name string) (int64, bool) {
	text, ok := o[name]
	if !ok || text == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(text, 10, 64)
	return n, err == nil
}

// List returns a repeatable option, which the parser joins with newlines.
func (o Options) List(name string) []string {
	if o[name] == "" {
		return nil
	}
	return strings.Split(o[name], "\n")
}

// With returns a copy with name set to value, as options.merge(name => value).
func (o Options) With(name, value string) Options {
	out := o.clone()
	out[name] = value
	return out
}

// Without returns a copy without the names, as options.without(*names).
func (o Options) Without(names ...string) Options {
	out := o.clone()
	for _, name := range names {
		delete(out, name)
	}
	return out
}

// Merge returns a copy with the values of other added, other winning.
func (o Options) Merge(other Options) Options {
	out := o.clone()
	for name, value := range other {
		out[name] = value
	}
	return out
}

func (o Options) clone() Options {
	out := Options{}
	for name, value := range o {
		out[name] = value
	}
	return out
}

// Describe returns the options as name=value pairs in name order, so tests can
// compare the options of an invocation.
func (o Options) Describe() string {
	names := make([]string, 0, len(o))
	for name := range o {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, len(names))
	for i, name := range names {
		parts[i] = name + "=" + o[name]
	}
	return strings.Join(parts, " ")
}
