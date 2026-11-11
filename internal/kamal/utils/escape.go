package utils

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// EscapeShellValue quotes value for the shell as Kamal::Utils.escape_shell_value
// does: each ASCII run goes through Ruby's String#dump, backticks get a backslash,
// and a $ gets one unless it starts a ${...} expansion. Non-ASCII runs stay as is.
//
//	utils.EscapeShellValue("$PWD")   // "\$PWD"
//	utils.EscapeShellValue("${PWD}") // "${PWD}"
func EscapeShellValue(value string) string {
	var out strings.Builder
	for _, run := range ASCIIRuns(value) {
		if !isASCII(run) {
			out.WriteString(run)
			continue
		}
		out.WriteString(escapeDollars(strings.ReplaceAll(RubyDump(run), "`", "\\`")))
	}
	return out.String()
}

// ASCIIRuns splits text into alternating runs of ASCII and non-ASCII characters, as
// the Ruby pattern /[\x00-\x7F]+|[^\x00-\x7F]+/ does.
func ASCIIRuns(text string) []string {
	var runs []string
	start := 0
	for i := 1; i <= len(text); i++ {
		if i == len(text) || (text[i] < 0x80) != (text[start] < 0x80) {
			runs = append(runs, text[start:i])
			start = i
		}
	}
	return runs
}

func isASCII(text string) bool { return text != "" && text[0] < 0x80 }

// escapeDollars adds a backslash before each $ that does not start ${...}, as the
// Ruby pattern /\$(?!{[^\}]*\})/ does.
func escapeDollars(text string) string {
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		if text[i] == '$' && !startsBraceExpansion(text[i+1:]) {
			out.WriteByte('\\')
		}
		out.WriteByte(text[i])
	}
	return out.String()
}

func startsBraceExpansion(rest string) bool {
	if !strings.HasPrefix(rest, "{") {
		return false
	}
	return strings.Contains(rest[1:], "}")
}

// RubyDump returns Ruby's String#dump of text: double quotes around it, and escapes
// for quotes, backslashes, control characters, and # before {, $, or @.
//
//	utils.RubyDump(`a"b`) // "a\"b"
func RubyDump(text string) string {
	var out strings.Builder
	out.WriteByte('"')
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		out.WriteString(dumpRune(r, text[i+size:]))
		i += size
	}
	out.WriteByte('"')
	return out.String()
}

var dumpEscapes = map[rune]string{
	'"': `\"`, '\\': `\\`, '\n': `\n`, '\r': `\r`, '\t': `\t`, '\f': `\f`, '\v': `\v`,
	'\a': `\a`, '\b': `\b`, 0x1b: `\e`,
}

func dumpRune(r rune, rest string) string {
	if escaped, ok := dumpEscapes[r]; ok {
		return escaped
	}
	switch {
	case r == '#' && rest != "" && strings.ContainsRune("{$@", rune(rest[0])):
		return `\#`
	case r < 0x20 || r == 0x7f:
		return fmt.Sprintf(`\x%02X`, r)
	case r >= 0x80:
		return dumpUnicode(r)
	default:
		return string(r)
	}
}

func dumpUnicode(r rune) string {
	if r > 0xFFFF {
		return fmt.Sprintf(`\u{%X}`, r)
	}
	return fmt.Sprintf(`\u%04X`, r)
}
