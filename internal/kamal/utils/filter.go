package utils

import "strings"

// FilterSpecificItems returns the items that match any filter, in filter order and
// without duplicates, as Kamal::Utils.filter_specific_items does. A filter is a
// pattern for Ruby's File.fnmatch with File::FNM_EXTGLOB: *, ?, [abc], and {a,b}.
//
//	utils.FilterSpecificItems([]string{"web*"}, []string{"web", "workers"}) // [web]
func FilterSpecificItems(filters, items []string) []string {
	var matches []string
	seen := map[string]bool{}
	for _, filter := range filters {
		for _, item := range items {
			if !seen[item] && FNMatch(filter, item) {
				seen[item] = true
				matches = append(matches, item)
			}
		}
	}
	return matches
}

// FNMatch reports whether text matches pattern as File.fnmatch(pattern, text,
// File::FNM_EXTGLOB) does: braces expand first, * also matches /, and a leading dot
// in text needs a literal dot in the pattern.
func FNMatch(pattern, text string) bool {
	for _, expanded := range expandBraces(pattern) {
		if globMatch(expanded, text, true) {
			return true
		}
	}
	return false
}

// expandBraces turns "web-{1,2}" into "web-1" and "web-2". Nested braces expand from
// the outside in.
func expandBraces(pattern string) []string {
	open, close := braceBounds(pattern)
	if open < 0 {
		return []string{pattern}
	}
	prefix, suffix := pattern[:open], pattern[close+1:]
	var out []string
	for _, choice := range splitTopLevel(pattern[open+1 : close]) {
		out = append(out, expandBraces(prefix+choice+suffix)...)
	}
	return out
}

func braceBounds(pattern string) (int, int) {
	depth, open := 0, -1
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '\\':
			i++
		case '{':
			if depth == 0 {
				open = i
			}
			depth++
		case '}':
			depth--
			if depth == 0 && open >= 0 {
				return open, i
			}
		}
	}
	return -1, -1
}

func splitTopLevel(body string) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '{':
			depth++
		case '}':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, body[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, body[start:])
}

// globMatch matches *, ?, [...], and \ escapes. atStart tracks the leading-dot rule.
func globMatch(pattern, text string, atStart bool) bool {
	for pattern != "" {
		if atStart && hiddenStart(pattern, text) {
			return false
		}
		if pattern[0] == '*' {
			return starMatch(pattern[1:], text)
		}
		var ok bool
		if pattern, text, ok = globStep(pattern, text); !ok {
			return false
		}
		atStart = false
	}
	return text == ""
}

// hiddenStart reports a leading dot in text that the pattern does not spell out.
func hiddenStart(pattern, text string) bool {
	return strings.HasPrefix(text, ".") && pattern[0] != '.' && pattern[0] != '\\'
}

// globStep matches one ?, [...], or literal character.
func globStep(pattern, text string) (string, string, bool) {
	switch pattern[0] {
	case '?':
		if text == "" {
			return pattern, text, false
		}
		return pattern[1:], text[1:], true
	case '[':
		rest, ok := classMatch(pattern, text)
		if !ok {
			return pattern, text, false
		}
		return rest, text[1:], true
	default:
		return literalMatch(pattern, text)
	}
}

func starMatch(pattern, text string) bool {
	for i := 0; i <= len(text); i++ {
		if globMatch(pattern, text[i:], false) {
			return true
		}
	}
	return false
}

func literalMatch(pattern, text string) (string, string, bool) {
	if pattern[0] == '\\' && len(pattern) > 1 {
		pattern = pattern[1:]
	}
	if text == "" || pattern[0] != text[0] {
		return pattern, text, false
	}
	return pattern[1:], text[1:], true
}

// classMatch matches one character of text against a [...] class at the start of
// pattern and returns the pattern after the class.
func classMatch(pattern, text string) (string, bool) {
	end := strings.IndexByte(pattern[1:], ']')
	if end < 0 || text == "" {
		return pattern, false
	}
	class := pattern[1 : end+1]
	negate := strings.HasPrefix(class, "!") || strings.HasPrefix(class, "^")
	if negate {
		class = class[1:]
	}
	return pattern[end+2:], inClass(class, text[0]) != negate
}

func inClass(class string, c byte) bool {
	for i := 0; i < len(class); i++ {
		if i+2 < len(class) && class[i+1] == '-' {
			if class[i] <= c && c <= class[i+2] {
				return true
			}
			i += 2
			continue
		}
		if class[i] == c {
			return true
		}
	}
	return false
}
