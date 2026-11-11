package yamlord

import (
	"regexp"
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
		if symbolKeys || m.HasSymbolKeys() {
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

var (
	// nonWordStart is Psych's /^[^[:word:]][^"]*$/, where a word character is a
	// letter, a mark, a decimal digit, or a connector such as "_".
	nonWordStart = regexp.MustCompile(`^[^\p{L}\p{M}\p{Nd}\p{Pc}][^"]*$`)
	octalLike    = regexp.MustCompile(`^0[0-7]*[89]`)
)

// quoteString picks the style of a string scalar by the rules of Psych's
// YAMLTree#visit_String, in its order: "--log-opt" is double quoted, "999" is
// single quoted, and "dhh/app" stays plain.
func quoteString(text string) string {
	switch {
	case text == "<<":
		// Psych tags it, so YAML does not read it back as a merge key.
		return "!!str '<<'"
	case text == "y" || text == "Y" || text == "n" || text == "N":
		return `"` + text + `"`
	case needsDoubleQuotes(text) || nonWordStart.MatchString(text):
		return strconv.Quote(text)
	case text == "" || tokenize(text).Kind() != String || octalLike.MatchString(text) || !plainAllowed(text):
		return "'" + strings.ReplaceAll(text, "'", "''") + "'"
	default:
		return text
	}
}

func needsDoubleQuotes(text string) bool {
	for _, r := range text {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// plainAllowed reports whether libyaml can emit text as a plain scalar. When it
// cannot, libyaml falls back to single quotes.
func plainAllowed(text string) bool {
	if strings.TrimSpace(text) != text || strings.ContainsAny(text[:1], "-?:,[]{}#&*!|>'\"%@`") {
		return false
	}
	return !strings.Contains(text, ": ") && !strings.Contains(text, " #") && !strings.HasSuffix(text, ":")
}

// SortedKeys returns the keys of m in sorted order.
func SortedKeys(m *Ordered) []string {
	keys := m.Keys()
	sort.Strings(keys)
	return keys
}
