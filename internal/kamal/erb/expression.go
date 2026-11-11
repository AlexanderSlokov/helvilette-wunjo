package erb

import (
	"errors"
	"fmt"
	"strings"
)

// errUnsupported marks an expression outside the subset. The renderer turns it into
// the error that names the file, the line, and the expression.
var errUnsupported = errors.New("unsupported ERB expression")

// evalExpression evaluates the body of an output tag, such as `ENV["NAME"]`. It
// parses the whole expression before it reads the environment, so trailing Ruby
// such as a block after ENV.fetch reports as unsupported.
func evalExpression(expr string, lookup LookupFunc) (string, error) {
	p := &exprParser{src: expr}
	parsed, err := p.expression()
	if err != nil {
		return "", err
	}
	p.skipSpace()
	if !p.done() {
		return "", errUnsupported
	}
	return parsed.evaluate(lookup)
}

// exprKind names the three forms of the subset.
type exprKind int

const (
	literalExpr exprKind = iota
	indexExpr
	fetchExpr
)

// expression is one parsed expression of the subset.
type expression struct {
	kind        exprKind
	text        string
	fallback    string
	hasFallback bool
}

// evaluate returns the value of the expression. ENV["NAME"] renders an unset variable
// as an empty string, as Ruby's nil.to_s does.
func (e expression) evaluate(lookup LookupFunc) (string, error) {
	switch e.kind {
	case indexExpr:
		value, _ := lookup(e.text)
		return value, nil
	case fetchExpr:
		return resolveFetch(e, lookup)
	default:
		return e.text, nil
	}
}

func resolveFetch(e expression, lookup LookupFunc) (string, error) {
	if value, ok := lookup(e.text); ok {
		return value, nil
	}
	if e.hasFallback {
		return e.fallback, nil
	}
	return "", fmt.Errorf("ENV.fetch(%q): environment variable %s is not set: set it, or give a default with ENV.fetch(%q, \"default\")", e.text, e.text, e.text)
}

// exprParser reads one expression of the subset from left to right.
type exprParser struct {
	src string
	pos int
}

func (p *exprParser) expression() (expression, error) {
	switch {
	case p.consume("ENV.fetch"):
		return p.fetch()
	case p.consume("ENV"):
		return p.index()
	default:
		text, err := p.stringLiteral()
		return expression{kind: literalExpr, text: text}, err
	}
}

// index parses ENV["NAME"].
func (p *exprParser) index() (expression, error) {
	if !p.consumeToken("[") {
		return expression{}, errUnsupported
	}
	name, err := p.spacedString()
	if err != nil || !p.consumeToken("]") {
		return expression{}, errUnsupported
	}
	return expression{kind: indexExpr, text: name}, nil
}

// fetch parses ENV.fetch("NAME") and ENV.fetch("NAME", "default").
func (p *exprParser) fetch() (expression, error) {
	if !p.consumeToken("(") {
		return expression{}, errUnsupported
	}
	name, err := p.spacedString()
	if err != nil {
		return expression{}, err
	}
	fallback, hasFallback, err := p.fetchDefault()
	return expression{kind: fetchExpr, text: name, fallback: fallback, hasFallback: hasFallback}, err
}

func (p *exprParser) fetchDefault() (string, bool, error) {
	if p.consumeToken(")") {
		return "", false, nil
	}
	if !p.consumeToken(",") {
		return "", false, errUnsupported
	}
	fallback, err := p.spacedString()
	if err != nil || !p.consumeToken(")") {
		return "", false, errUnsupported
	}
	return fallback, true, nil
}

func (p *exprParser) spacedString() (string, error) {
	p.skipSpace()
	value, err := p.stringLiteral()
	p.skipSpace()
	return value, err
}

func (p *exprParser) stringLiteral() (string, error) {
	switch {
	case p.consume(`"`):
		return p.quoted('"', doubleQuoteEscape)
	case p.consume("'"):
		return p.quoted('\'', singleQuoteEscape)
	default:
		return "", errUnsupported
	}
}

// quoted reads the rest of a string literal up to the closing quote.
func (p *exprParser) quoted(quote byte, escape func(next byte) (string, bool)) (string, error) {
	var out strings.Builder
	for !p.done() {
		c := p.src[p.pos]
		p.pos++
		if c == quote {
			return out.String(), nil
		}
		if err := p.appendChar(&out, c, quote, escape); err != nil {
			return "", err
		}
	}
	return "", errUnsupported
}

func (p *exprParser) appendChar(out *strings.Builder, c, quote byte, escape func(byte) (string, bool)) error {
	if quote == '"' && c == '#' && p.interpolates() {
		return errUnsupported
	}
	if c != '\\' || p.done() {
		out.WriteByte(c)
		return nil
	}
	text, ok := escape(p.src[p.pos])
	if !ok {
		return errUnsupported
	}
	p.pos++
	out.WriteString(text)
	return nil
}

// interpolates reports whether a # inside a double-quoted string starts Ruby
// interpolation, which the subset cannot evaluate.
func (p *exprParser) interpolates() bool {
	return !p.done() && strings.ContainsRune("{@$", rune(p.src[p.pos]))
}

// doubleQuoteEscape maps the escapes of a double-quoted Ruby string that the subset
// supports. Other escapes, such as \u, are outside the subset.
func doubleQuoteEscape(next byte) (string, bool) {
	text, ok := map[byte]string{'\\': `\`, '"': `"`, 'n': "\n", 't': "\t", 'r': "\r", '#': "#"}[next]
	return text, ok
}

// singleQuoteEscape follows Ruby: only \' and \\ are escapes, and any other
// backslash stays in the text.
func singleQuoteEscape(next byte) (string, bool) {
	if next == '\'' || next == '\\' {
		return string(next), true
	}
	return `\` + string(next), true
}

func (p *exprParser) consume(prefix string) bool {
	if !strings.HasPrefix(p.src[p.pos:], prefix) {
		return false
	}
	p.pos += len(prefix)
	return true
}

func (p *exprParser) consumeToken(token string) bool {
	p.skipSpace()
	return p.consume(token)
}

func (p *exprParser) skipSpace() {
	for !p.done() && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t') {
		p.pos++
	}
}

func (p *exprParser) done() bool { return p.pos >= len(p.src) }
