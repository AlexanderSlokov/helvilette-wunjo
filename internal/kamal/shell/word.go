// Package shell holds the words of a command line and Ruby's shell quoting rules.
// Kamal builds commands as arrays of words, some of them sensitive, and SSHKit
// prints the redacted form in logs. A Line keeps both forms.
//
// Ports lib/kamal/utils/sensitive.rb and the parts of Ruby's Shellwords that Kamal
// uses at Kamal 2.12.0.
package shell

import "strings"

// DefaultRedaction replaces a sensitive word in logs.
const DefaultRedaction = "[REDACTED]"

// Word is one word of a command line.
type Word struct {
	text      string
	redaction string
	sensitive bool
}

// Plain returns a word that logs show as is, for example shell.Plain("docker").
func Plain(text string) Word { return Word{text: text} }

// Sensitive returns a word that logs show as redaction, for example
// shell.Sensitive("PASSWORD=hunter2", "PASSWORD=[REDACTED]").
func Sensitive(text, redaction string) Word {
	return Word{text: text, redaction: redaction, sensitive: true}
}

// String returns the text that runs, as Kamal::Utils::Sensitive#to_s does.
func (w Word) String() string { return w.text }

// Redacted returns the text that logs show.
func (w Word) Redacted() string {
	if w.sensitive {
		return w.redaction
	}
	return w.text
}

// IsSensitive reports whether logs hide the word.
func (w Word) IsSensitive() bool { return w.sensitive }

// Line is a command line as a list of words.
type Line []Word

// Words returns a Line of plain words, for example shell.Words("docker", "ps").
func Words(texts ...string) Line {
	line := make(Line, len(texts))
	for i, text := range texts {
		line[i] = Plain(text)
	}
	return line
}

// String joins the words with spaces, as Ruby's Array#join(" ") does.
func (l Line) String() string { return l.join(Word.String) }

// Redacted joins the redacted words with spaces.
func (l Line) Redacted() string { return l.join(Word.Redacted) }

func (l Line) join(form func(Word) string) string {
	parts := make([]string, len(l))
	for i, word := range l {
		parts[i] = form(word)
	}
	return strings.Join(parts, " ")
}

// Concat returns the words of all lines in order, as Ruby's [*a, *b].
func Concat(lines ...Line) Line {
	var out Line
	for _, line := range lines {
		out = append(out, line...)
	}
	return out
}
