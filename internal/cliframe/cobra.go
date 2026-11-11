package cliframe

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Execute runs the command tree with the given arguments, without the program name.
//
//	err := cliframe.Execute(root, os.Args[1:], os.Stdout, os.Stderr)
func Execute(root *Command, args []string, stdout, stderr io.Writer) error {
	cmd := toCobra(root, stdout, stderr)
	cmd.SetArgs(normalizeArgs(root, args))
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.CompletionOptions.DisableDefaultCmd = true
	// Kamal uses -h for --hosts, so help takes only the long form.
	cmd.PersistentFlags().Bool("help", false, "Show help for a command")
	return cmd.Execute()
}

func toCobra(spec *Command, stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     strings.TrimSpace(spec.Name + " " + spec.ArgsUsage),
		Short:   spec.Short,
		Long:    spec.Long,
		Aliases: spec.Aliases,
		Hidden:  spec.Hidden,
		Args:    argRule(spec),
	}
	cmd.SetGlobalNormalizationFunc(underscoreToDash)
	addFlags(cmd.Flags(), spec.Flags)
	addFlags(cmd.PersistentFlags(), spec.GlobalFlags)
	if spec.Run != nil {
		cmd.RunE = runner(spec.Run, stdout, stderr)
	}
	for _, child := range spec.Children {
		cmd.AddCommand(toCobra(child, stdout, stderr))
	}
	return cmd
}

func runner(run func(*Invocation) error, stdout, stderr io.Writer) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		inv := &Invocation{Args: args, Stdout: stdout, Stderr: stderr, flags: pflagReader{cmd.Flags()}}
		return run(inv)
	}
}

func argRule(spec *Command) cobra.PositionalArgs {
	if spec.MaxArgs == AnyArgs {
		return cobra.MinimumNArgs(spec.MinArgs)
	}
	return cobra.RangeArgs(spec.MinArgs, spec.MaxArgs)
}

// underscoreToDash accepts --skip_push as --skip-push, as Thor does.
func underscoreToDash(_ *pflag.FlagSet, name string) pflag.NormalizedName {
	return pflag.NormalizedName(strings.ReplaceAll(name, "_", "-"))
}

func addFlags(set *pflag.FlagSet, flags []Flag) {
	for _, flag := range flags {
		addFlag(set, flag)
	}
}

func addFlag(set *pflag.FlagSet, flag Flag) {
	switch flag.Kind {
	case BoolFlag:
		set.BoolP(flag.Name, flag.Short, flag.Default == "true", flag.Usage)
	case IntFlag:
		value, _ := strconv.Atoi(flag.Default)
		set.IntP(flag.Name, flag.Short, value, flag.Usage)
	case StringListFlag:
		set.StringArrayP(flag.Name, flag.Short, nil, flag.Usage)
	default:
		set.StringP(flag.Name, flag.Short, flag.Default, flag.Usage)
	}
}

// pflagReader reads values from a parsed pflag set. A lookup of an undeclared flag is
// a programming error in the command tree, so it panics with the flag name.
type pflagReader struct{ set *pflag.FlagSet }

func (r pflagReader) boolValue(name string) bool {
	value, err := r.set.GetBool(name)
	mustDeclare(name, err)
	return value
}

func (r pflagReader) stringValue(name string) string {
	value, err := r.set.GetString(name)
	mustDeclare(name, err)
	return value
}

func (r pflagReader) intValue(name string) int {
	value, err := r.set.GetInt(name)
	mustDeclare(name, err)
	return value
}

func (r pflagReader) stringsValue(name string) []string {
	value, err := r.set.GetStringArray(name)
	mustDeclare(name, err)
	return value
}

func (r pflagReader) changed(name string) bool { return r.set.Changed(name) }

func mustDeclare(name string, err error) {
	if err != nil {
		panic(fmt.Sprintf("cliframe: flag %q is not declared with the requested kind: %v", name, err))
	}
}
