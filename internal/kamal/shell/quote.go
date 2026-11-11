package shell

import (
	"fmt"
	"strings"
)

// Escape quotes text for a POSIX shell as Ruby's String#shellescape does: an empty
// text becomes ”, each character outside A-Za-z0-9_-.,:+/@ and newline gets a
// backslash, and a newline becomes '\n'.
//
//	shell.Escape("it's") // it\'s
func Escape(text string) string {
	if text == "" {
		return "''"
	}
	var out strings.Builder
	for _, r := range text {
		switch {
		case r == '\n':
			out.WriteString("'\n'")
		case isShellSafe(r):
			out.WriteRune(r)
		default:
			out.WriteByte('\\')
			out.WriteRune(r)
		}
	}
	return out.String()
}

func isShellSafe(r rune) bool {
	return r < 128 && (r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("_-.,:+/@", r))
}

// Split breaks a line into words as Ruby's String#shellsplit does, with single
// quotes, double quotes, and backslash escapes.
//
//	words, err := shell.Split(`secrets fetch "a b"`) // [secrets fetch a b]
func Split(line string) ([]string, error) {
	s := &splitter{src: line}
	return s.run()
}

type splitter struct {
	src   string
	pos   int
	words []string
	field strings.Builder
	open  bool
}

func (s *splitter) run() ([]string, error) {
	for {
		s.skipSpace()
		if s.done() {
			s.flush()
			return s.words, nil
		}
		if err := s.readToken(); err != nil {
			return nil, err
		}
		if s.done() || isSpace(s.src[s.pos]) {
			s.flush()
		}
	}
}

func (s *splitter) readToken() error {
	start := s.pos
	s.open = true
	switch c := s.src[s.pos]; c {
	case '\'':
		return s.quoted(start, '\'', func(text string) string { return text })
	case '"':
		return s.quoted(start, '"', unescapeDoubleQuoted)
	case '\\':
		s.escaped()
	default:
		s.bare()
	}
	return nil
}

func (s *splitter) bare() {
	for !s.done() && !isSpace(s.src[s.pos]) && !strings.ContainsRune(`\'"`, rune(s.src[s.pos])) {
		s.field.WriteByte(s.src[s.pos])
		s.pos++
	}
}

func (s *splitter) escaped() {
	s.pos++
	if s.done() {
		s.field.WriteByte('\\')
		return
	}
	s.field.WriteByte(s.src[s.pos])
	s.pos++
}

func (s *splitter) quoted(start int, quote byte, unescape func(string) string) error {
	end := closingQuote(s.src, s.pos+1, quote)
	if end < 0 {
		return unmatchedQuote(s.src, start)
	}
	s.field.WriteString(unescape(s.src[s.pos+1 : end]))
	s.pos = end + 1
	return nil
}

// closingQuote finds the closing quote; inside double quotes a backslash escapes the
// next character.
func closingQuote(src string, from int, quote byte) int {
	for i := from; i < len(src); i++ {
		if quote == '"' && src[i] == '\\' {
			i++
			continue
		}
		if src[i] == quote {
			return i
		}
	}
	return -1
}

// unmatchedQuote names the position of the quote, as Ruby does with the text of the
// failing match: the quote character itself.
func unmatchedQuote(src string, start int) error {
	shown := src[start : start+1]
	if start > 0 {
		shown = "..." + shown
	}
	return fmt.Errorf("Unmatched quote at %d: %s", start, shown)
}

// unescapeDoubleQuoted keeps a backslash except before $ ` " \ and newline, as POSIX
// double quotes do.
func unescapeDoubleQuoted(text string) string {
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		if text[i] == '\\' && i+1 < len(text) && strings.ContainsRune("$`\"\\\n", rune(text[i+1])) {
			i++
		}
		out.WriteByte(text[i])
	}
	return out.String()
}

func (s *splitter) flush() {
	if s.open {
		s.words = append(s.words, s.field.String())
	}
	s.field.Reset()
	s.open = false
}

func (s *splitter) skipSpace() {
	for !s.done() && isSpace(s.src[s.pos]) {
		s.pos++
	}
}

func (s *splitter) done() bool { return s.pos >= len(s.src) }

func isSpace(c byte) bool { return strings.IndexByte(" \t\n\v\f\r", c) >= 0 }
