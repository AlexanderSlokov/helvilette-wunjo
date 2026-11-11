package yamlord

import (
	"sort"
	"strconv"
	"strings"
)

// DumpWithSymbolKeys writes v as YAML in the layout of Psych's to_yaml, with the
// top-level keys as symbols (":roles:"), as `kamal config` prints
// Configuration#to_h.
//
//	text := yamlord.DumpWithSymbolKeys(yamlord.NewMap(h)) // "---\n:roles:\n- web\n..."
func DumpWithSymbolKeys(v *Value) string {
	var out strings.Builder
	out.WriteString("---")
	writeValue(&out, v, 0, true)
	out.WriteString("\n")
	return out.String()
}

func writeValue(out *strings.Builder, v *Value, indent int, symbolKeys bool) {
	switch {
	case v.Kind() == Map && v.Map().Len() > 0:
		writeMap(out, v.Map(), indent, symbolKeys)
	case v.Kind() == List && len(v.List()) > 0:
		writeList(out, v.List(), indent)
	default:
		out.WriteString(" " + scalar(v))
	}
}

func writeMap(out *strings.Builder, m *Ordered, indent int, symbolKeys bool) {
	for _, key := range m.Keys() {
		out.WriteString("\n" + strings.Repeat("  ", indent))
		if symbolKeys {
			out.WriteString(":")
		}
		out.WriteString(quoteKey(key) + ":")
		writeNested(out, m.At(key), indent)
	}
}

func writeNested(out *strings.Builder, v *Value, indent int) {
	if v.Kind() == List && len(v.List()) > 0 {
		writeList(out, v.List(), indent)
		return
	}
	writeValue(out, v, indent+1, false)
}

func writeList(out *strings.Builder, items []*Value, indent int) {
	for _, item := range items {
		out.WriteString("\n" + strings.Repeat("  ", indent) + "-")
		writeValue(out, item, indent+1, false)
	}
}

func scalar(v *Value) string {
	switch v.Kind() {
	case Null:
		return ""
	case Map:
		return "{}"
	case List:
		return "[]"
	case Symbol:
		return ":" + v.Text()
	case String, Date:
		return quoteString(v.Text())
	default:
		return v.ToS()
	}
}

func quoteKey(key string) string { return quoteString(key) }

// quoteString leaves a string plain when YAML reads it back as the same string,
// and quotes it otherwise: double quotes for control characters, single quotes
// for the rest.
func quoteString(text string) string {
	if needsDoubleQuotes(text) {
		return strconv.Quote(text)
	}
	if plainSafe(text) {
		return text
	}
	return "'" + strings.ReplaceAll(text, "'", "''") + "'"
}

func needsDoubleQuotes(text string) bool {
	for _, r := range text {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

func plainSafe(text string) bool {
	if text == "" || strings.TrimSpace(text) != text || strings.ContainsAny(text[:1], "-?:,[]{}#&*!|>'\"%@`") {
		return false
	}
	if strings.Contains(text, ": ") || strings.Contains(text, " #") || strings.HasSuffix(text, ":") {
		return false
	}
	return tokenize(text).Kind() == String
}

// SortedKeys returns the keys of m in sorted order.
func SortedKeys(m *Ordered) []string {
	keys := m.Keys()
	sort.Strings(keys)
	return keys
}
