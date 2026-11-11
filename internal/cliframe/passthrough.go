package cliframe

import "strings"

// normalizeArgs rewrites Thor-style arguments into a form cobra parses:
//
//   - "--flag false" for a boolean becomes "--flag=false";
//   - a greedy list flag repeats for each following word: "--x a b" is "--x a --x b";
//   - for a PassUnknownFlags command, declared flags move in front and every other
//     word goes after "--", so unknown flags stay positional arguments.
func normalizeArgs(root *Command, args []string) []string {
	path, rest := commandPath(root, args)
	prefix := args[:len(args)-len(rest)]
	known := knownFlags(path)
	flags, positional := splitFlags(rest, known)
	if !path[len(path)-1].PassUnknownFlags {
		return append(append(append([]string{}, prefix...), flags...), positional...)
	}
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

// knownFlags maps each declared flag spelling to its flag.
func knownFlags(path []*Command) map[string]Flag {
	known := map[string]Flag{"--help": {Name: "help", Kind: BoolFlag}}
	for i, cmd := range path {
		flags := cmd.GlobalFlags
		if i == len(path)-1 {
			flags = append(append([]Flag{}, flags...), cmd.Flags...)
		}
		for _, f := range flags {
			known["--"+f.Name], known["--"+strings.ReplaceAll(f.Name, "-", "_")] = f, f
			if f.Short != "" {
				known["-"+f.Short] = f
			}
		}
	}
	return known
}

// splitFlags separates the declared flags, with their values, from the other words.
func splitFlags(words []string, known map[string]Flag) ([]string, []string) {
	var flags, positional []string
	for i := 0; i < len(words); i++ {
		word := words[i]
		if word == "--" {
			return flags, append(positional, words[i+1:]...)
		}
		name, _, hasValue := strings.Cut(word, "=")
		f, ok := known[name]
		if !ok {
			positional = append(positional, word)
			continue
		}
		var taken []string
		taken, i = takeFlag(f, word, words, i, hasValue)
		flags = append(flags, taken...)
	}
	return flags, positional
}

// takeFlag returns the words of one flag starting at words[i] and the index of its
// last word.
func takeFlag(f Flag, word string, words []string, i int, hasValue bool) ([]string, int) {
	next := func() (string, bool) {
		if i+1 < len(words) {
			return words[i+1], true
		}
		return "", false
	}
	switch {
	case hasValue:
		return []string{word}, i
	case f.Kind == BoolFlag:
		if value, ok := next(); ok && (value == "true" || value == "false") {
			return []string{word + "=" + value}, i + 1
		}
		return []string{word}, i
	case f.Greedy:
		return greedyValues(word, words, i)
	default:
		if value, ok := next(); ok {
			return []string{word, value}, i + 1
		}
		return []string{word}, i
	}
}

func greedyValues(word string, words []string, i int) ([]string, int) {
	var out []string
	for i+1 < len(words) && !strings.HasPrefix(words[i+1], "-") {
		out = append(out, word, words[i+1])
		i++
	}
	if len(out) == 0 {
		return []string{word}, i
	}
	return out, i
}
