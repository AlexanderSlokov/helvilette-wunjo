// Package utils builds Docker arguments and quotes values for the shell, as
// Kamal::Utils does.
//
// Ports lib/kamal/utils.rb at Kamal 2.12.0.
package utils

import (
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// Attr is one key and value that argumentize or optionize turns into flags. A nil
// Value stands for Ruby's nil.
type Attr struct {
	Key   string
	Value *yamlord.Value
}

// AttrsOf returns the pairs of a Hash in order, for example AttrsOf(labels).
func AttrsOf(m *yamlord.Ordered) []Attr {
	attrs := make([]Attr, 0, m.Len())
	m.Each(func(key string, value *yamlord.Value) {
		attrs = append(attrs, Attr{Key: key, Value: value})
	})
	return attrs
}

// Keys returns attributes without values, as Ruby's Array(["a", "b"]) gives to
// argumentize, for example Keys("/data:/data").
func Keys(keys ...string) []Attr {
	attrs := make([]Attr, len(keys))
	for i, key := range keys {
		attrs[i] = Attr{Key: key}
	}
	return attrs
}

// KeysOf returns Array(value) as attributes without values: one per item of an
// Array, one for a scalar, and none for nil.
func KeysOf(value *yamlord.Value) []Attr {
	return Keys(value.StringList()...)
}

// Argumentize returns the argument before each attribute, as
// Kamal::Utils.argumentize does: "key=value" with the value escaped, "key=false"
// for false, and the bare key for a blank value.
//
//	utils.Argumentize("--label", utils.AttrsOf(labels), false) // --label service="app"
func Argumentize(argument string, attrs []Attr, sensitive bool) shell.Line {
	var line shell.Line
	for _, attr := range attrs {
		line = append(line, shell.Plain(argument), argumentWord(attr, sensitive))
	}
	return line
}

func argumentWord(attr Attr, sensitive bool) shell.Word {
	switch {
	case attr.Value.Present():
		text := attr.Key + "=" + EscapeShellValue(attr.Value.ToS())
		if sensitive {
			return shell.Sensitive(text, attr.Key+"=[REDACTED]")
		}
		return shell.Plain(text)
	case attr.Value.IsFalse():
		return shell.Plain(attr.Key + "=false")
	default:
		return shell.Plain(attr.Key)
	}
}

// Optionize returns "--key value" pairs, with "--key" alone for true, as
// Kamal::Utils.optionize does without with:.
//
//	utils.Optionize(utils.AttrsOf(options)) // --memory "1g"
func Optionize(attrs []Attr) shell.Line {
	var line shell.Line
	for _, attr := range flattenArgs(attrs) {
		line = append(line, shell.Plain("--"+attr.Key))
		if !attr.Value.IsTrue() {
			line = append(line, shell.Plain(EscapeShellValue(attr.Value.ToS())))
		}
	}
	return line
}

// OptionizeWith returns "--key<with>value" words, with "--key" alone for true, as
// Kamal::Utils.optionize(args, with: "=") does.
func OptionizeWith(attrs []Attr, with string) shell.Line {
	var line shell.Line
	for _, attr := range flattenArgs(attrs) {
		word := "--" + attr.Key
		if !attr.Value.IsTrue() {
			word += with + EscapeShellValue(attr.Value.ToS())
		}
		line = append(line, shell.Plain(word))
	}
	return line
}

// flattenArgs gives each item of an Array value its own pair, as
// Kamal::Utils.flatten_args does.
func flattenArgs(attrs []Attr) []Attr {
	var out []Attr
	for _, attr := range attrs {
		out = append(out, flattenAttr(attr)...)
	}
	return out
}

func flattenAttr(attr Attr) []Attr {
	switch attr.Value.Kind() {
	case yamlord.List:
		return pairsFor(attr.Key, attr.Value.List())
	case yamlord.Map:
		return hashEntries(attr.Key, attr.Value.Map())
	default:
		return []Attr{attr}
	}
}

func pairsFor(key string, items []*yamlord.Value) []Attr {
	out := make([]Attr, len(items))
	for i, item := range items {
		out[i] = Attr{Key: key, Value: item}
	}
	return out
}

// hashEntries follows Ruby's Hash#map, which yields [key, value] pairs.
func hashEntries(key string, m *yamlord.Ordered) []Attr {
	var out []Attr
	m.Each(func(k string, v *yamlord.Value) {
		out = append(out, Attr{Key: key, Value: yamlord.NewList(yamlord.NewString(k), v)})
	})
	return out
}
