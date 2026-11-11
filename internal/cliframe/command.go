// Package cliframe describes Wunjo's command tree without exposing the command-line
// library underneath. The rest of Wunjo builds Command values; only this package
// imports spf13/cobra, as AGENTS.md requires for third-party libraries.
package cliframe

import "io"

// FlagKind names the value type of a flag.
type FlagKind int

const (
	// BoolFlag is a flag without a value, such as --verbose.
	BoolFlag FlagKind = iota
	// StringFlag takes one text value, such as --hosts=web-1.
	StringFlag
	// IntFlag takes one integer value, such as --lines=100.
	IntFlag
	// StringListFlag takes a value each time it appears, such as -e A:1 -e B:2.
	StringListFlag
)

// Flag describes one command-line option.
//
//	cliframe.Flag{Name: "skip-push", Short: "P", Kind: cliframe.BoolFlag, Usage: "Skip image build and push"}
type Flag struct {
	Name    string
	Short   string
	Usage   string
	Kind    FlagKind
	Default string
	// Greedy makes a StringListFlag take every following word up to the next flag,
	// as Thor's array options do: --docker-options a=1 b=2.
	Greedy bool
	// Pairs makes a StringListFlag a Thor hash option: it takes the following
	// KEY:VALUE words, a repeated key is an error, and a repeated flag replaces
	// the earlier pairs. For example, "-e A:1 B:2 ruby -v" sets A and B.
	Pairs bool
}

// ParseError is an argument that Thor rejects before the command runs. Thor
// prints its message without an error class.
type ParseError struct{ Message string }

func (e *ParseError) Error() string { return e.Message }

// AnyArgs as Command.MaxArgs accepts any number of positional arguments.
const AnyArgs = -1

// Command describes one command and its subcommands. A Command with zero MinArgs and
// MaxArgs accepts no positional arguments, as a Thor command without parameters does.
//
//	root := &cliframe.Command{Name: "wunjo", Children: []*cliframe.Command{version}}
type Command struct {
	Name        string
	ArgsUsage   string
	Short       string
	Long        string
	Aliases     []string
	Hidden      bool
	MinArgs     int
	MaxArgs     int
	Flags       []Flag
	GlobalFlags []Flag
	Children    []*Command
	Run         func(inv *Invocation) error
	// PassUnknownFlags keeps flags the command does not declare as positional
	// arguments, as Thor does for `exec date -j`. Declared flags still parse.
	PassUnknownFlags bool
}

// Invocation carries the positional arguments, the parsed flags, and the output
// streams of one command run.
type Invocation struct {
	Args   []string
	Stdout io.Writer
	Stderr io.Writer
	flags  flagReader
}

// flagReader reads parsed flag values by long name.
type flagReader interface {
	boolValue(name string) bool
	stringValue(name string) string
	intValue(name string) int
	stringsValue(name string) []string
	changed(name string) bool
}

// Bool returns the value of a BoolFlag, for example inv.Bool("verbose").
func (inv *Invocation) Bool(name string) bool { return inv.flags.boolValue(name) }

// String returns the value of a StringFlag, for example inv.String("hosts").
func (inv *Invocation) String(name string) string { return inv.flags.stringValue(name) }

// Int returns the value of an IntFlag, for example inv.Int("lines").
func (inv *Invocation) Int(name string) int { return inv.flags.intValue(name) }

// Strings returns the values of a StringListFlag, for example inv.Strings("env").
func (inv *Invocation) Strings(name string) []string { return inv.flags.stringsValue(name) }

// Changed reports whether the user set the flag, for example inv.Changed("hosts").
func (inv *Invocation) Changed(name string) bool { return inv.flags.changed(name) }
