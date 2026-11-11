package cliframe

import (
	"fmt"
	"sort"
	"strings"
)

// normalizeArgs rewrites Thor-style arguments into a form cobra parses:
//
//   - "--flag false" for a boolean becomes "--flag=false";
//   - a greedy list flag repeats for each following word: "--x a b" is "--x a --x b";
//   - a pairs flag takes the following KEY:VALUE words, and its last use wins;
//   - for a PassUnknownFlags command, declared flags move in front and every other
//     word goes after "--", so unknown flags stay positional arguments.
func normalizeArgs(root *Command, args []string) ([]string, error) {
	path, rest := commandPath(root, args)
	prefix := args[:len(args)-len(rest)]
	split := &flagSplitter{known: knownFlags(path), pairs: map[string][]string{}}
	if err := split.run(rest); err != nil {
		return nil, err
	}
	out := append(append([]string{}, prefix...), split.allFlags()...)
	if !path[len(path)-1].PassUnknownFlags {
		return append(out, split.positional...), nil
	}
	return append(append(out, "--"), split.positional...), nil
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

// flagSplitter separates the declared flags, with their values, from the other
// words. pairs holds the words of the last use of each pairs flag.
type flagSplitter struct {
	known      map[string]Flag
	flags      []string
	positional []string
	pairs      map[string][]string
}

func (s *flagSplitter) run(words []string) error {
	for i := 0; i < len(words); i++ {
		word := words[i]
		if word == "--" {
			s.positional = append(s.positional, words[i+1:]...)
			return nil
		}
		next, err := s.take(word, words, i)
		if err != nil {
			return err
		}
		i = next
	}
	return nil
}

// take consumes the word at words[i] and its values, and returns the index of the
// last word it consumed.
func (s *flagSplitter) take(word string, words []string, i int) (int, error) {
	name, value, hasValue := strings.Cut(word, "=")
	f, ok := s.known[name]
	switch {
	case !ok:
		s.positional = append(s.positional, word)
		return i, nil
	case f.Pairs:
		pairs, next, err := pairValues(f, value, hasValue, words, i)
		s.pairs["--"+f.Name] = pairs
		return next, err
	default:
		taken, next := takeFlag(f, word, words, i, hasValue)
		s.flags = append(s.flags, taken...)
		return next, nil
	}
}

// allFlags returns the flags with each pairs flag repeated for its pairs.
func (s *flagSplitter) allFlags() []string {
	out := append([]string{}, s.flags...)
	names := make([]string, 0, len(s.pairs))
	for name := range s.pairs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, pair := range s.pairs[name] {
			out = append(out, name+"="+pair)
		}
	}
	return out
}

// pairValues takes the KEY:VALUE words after a pairs flag, as Thor's parse_hash
// does, and returns the index of the last word it took.
func pairValues(f Flag, value string, hasValue bool, words []string, i int) ([]string, int, error) {
	candidates := words[i+1:]
	if hasValue {
		candidates = append([]string{value}, candidates...)
	}
	pairs, err := leadingPairs(f, candidates)
	if hasValue {
		return pairs, i + len(pairs) - 1, err
	}
	return pairs, i + len(pairs), err
}

// leadingPairs returns the KEY:VALUE words at the start of words and rejects a
// key that appears twice, with Thor's message.
func leadingPairs(f Flag, words []string) ([]string, error) {
	seen := map[string]string{}
	var pairs []string
	for _, word := range words {
		key, value, isPair := strings.Cut(word, ":")
		if !isPair || isSwitch(word) {
			break
		}
		if previous, dup := seen[key]; dup {
			return nil, &ParseError{Message: fmt.Sprintf("You can't specify '%s' more than once in option '--%s'; got %s:%s and %s:%s", key, f.Name, key, previous, key, value)}
		}
		seen[key] = value
		pairs = append(pairs, word)
	}
	return pairs, nil
}

// isSwitch reports whether word looks like a flag, as Thor's /^-{1,2}\S+/.
func isSwitch(word string) bool {
	return strings.HasPrefix(word, "-") && len(strings.TrimLeft(word, "-")) > 0 && len(word)-len(strings.TrimLeft(word, "-")) <= 2
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
	case hasValue && f.Greedy:
		return greedyAfterValue(word, words, i)
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

// greedyAfterValue keeps "--x=a" and takes the words after it, as Thor puts a back
// on the arguments and reads the array from there: "--x=a b" is "--x=a --x b".
func greedyAfterValue(word string, words []string, i int) ([]string, int) {
	name, _, _ := strings.Cut(word, "=")
	rest, last := greedyValues(name, words, i)
	if last == i {
		return []string{word}, i
	}
	return append([]string{word}, rest...), last
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
