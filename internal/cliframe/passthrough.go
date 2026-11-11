package cliframe

import "strings"

// normalizeArgs moves the declared flags of a PassUnknownFlags command in front and
// every other word after "--", so cobra parses the declared flags and keeps the rest,
// unknown flags included, as positional arguments in their order.
func normalizeArgs(root *Command, args []string) []string {
	path, rest := commandPath(root, args)
	cmd := path[len(path)-1]
	if !cmd.PassUnknownFlags {
		return args
	}
	known := knownFlags(path)
	prefix := args[:len(args)-len(rest)]
	flags, positional := splitFlags(rest, known)
	out := append(append([]string{}, prefix...), flags...)
	return append(append(out, "--"), positional...)
}

// commandPath follows command names from args and returns the commands and the
// words after the last name.
func commandPath(root *Command, args []string) ([]*Command, []string) {
	path := []*Command{root}
	for i, arg := range args {
		child := childNamed(path[len(path)-1], arg)
		if child == nil {
			return path, args[i:]
		}
		path = append(path, child)
	}
	return path, nil
}

func childNamed(cmd *Command, name string) *Command {
	for _, child := range cmd.Children {
		if child.Name == name {
			return child
		}
	}
	return nil
}

// knownFlags maps each declared flag spelling to whether it takes a value.
func knownFlags(path []*Command) map[string]bool {
	known := map[string]bool{"--help": false}
	for i, cmd := range path {
		flags := cmd.GlobalFlags
		if i == len(path)-1 {
			flags = append(append([]Flag{}, flags...), cmd.Flags...)
		}
		for _, f := range flags {
			takesValue := f.Kind != BoolFlag
			known["--"+f.Name], known["--"+strings.ReplaceAll(f.Name, "-", "_")] = takesValue, takesValue
			if f.Short != "" {
				known["-"+f.Short] = takesValue
			}
		}
	}
	return known
}

func splitFlags(words []string, known map[string]bool) ([]string, []string) {
	var flags, positional []string
	for i := 0; i < len(words); i++ {
		word := words[i]
		if word == "--" {
			return flags, append(positional, words[i+1:]...)
		}
		name, _, hasValue := strings.Cut(word, "=")
		takesValue, ok := known[name]
		if !ok {
			positional = append(positional, word)
			continue
		}
		flags = append(flags, word)
		if takesValue && !hasValue && i+1 < len(words) {
			flags = append(flags, words[i+1])
			i++
		}
	}
	return flags, positional
}
