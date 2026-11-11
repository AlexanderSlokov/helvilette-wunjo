// Package erb renders the subset of Embedded Ruby (ERB) that Wunjo supports in
// config/deploy.yml and in accessory files with the .erb extension. Go cannot run
// Ruby, so any expression outside the subset stops rendering with an error that names
// the file, the line, and the expression. RFC 0001, section "ERB in deploy.yml",
// defines the subset.
//
// Ports the ERB.new(template, trim_mode: "-").result call in
// lib/kamal/configuration.rb (line 42) and render_dynamic_erb in
// lib/kamal/configuration/accessory.rb (line 164) at Kamal 2.12.0.
package erb

import (
	"fmt"
	"strings"
)

// LookupFunc returns the value of an environment variable and whether it is set.
// os.LookupEnv satisfies it.
type LookupFunc func(name string) (string, bool)

// Options configures one render.
type Options struct {
	// File names the template in error messages, for example "config/deploy.yml".
	File string
	// TrimMode enables the <%- and -%> markers, as Kamal's trim_mode: "-" does.
	// Kamal renders deploy files with it and accessory files without it.
	TrimMode bool
	// Lookup reads environment variables for ENV["NAME"] and ENV.fetch.
	Lookup LookupFunc
}

// Render renders template and returns the result.
//
//	out, err := erb.Render(`user: <%= ENV["DB_USER"] %>`, erb.Options{File: "config/deploy.yml", TrimMode: true, Lookup: os.LookupEnv})
func Render(template string, opts Options) (string, error) {
	r := &renderer{src: template, opts: opts}
	if err := r.run(); err != nil {
		return "", err
	}
	return r.out.String(), nil
}

// renderer walks the template once, copying text and evaluating tags.
type renderer struct {
	src  string
	pos  int
	out  strings.Builder
	opts Options
}

func (r *renderer) run() error {
	for {
		open := strings.Index(r.src[r.pos:], "<%")
		if open < 0 {
			r.out.WriteString(r.src[r.pos:])
			return nil
		}
		r.out.WriteString(r.src[r.pos : r.pos+open])
		r.pos += open
		if err := r.tag(); err != nil {
			return err
		}
	}
}

// tag handles one tag that starts at r.pos.
func (r *renderer) tag() error {
	if strings.HasPrefix(r.src[r.pos:], "<%%") {
		r.out.WriteString("<%")
		r.pos += 3
		return nil
	}
	line := r.lineAt(r.pos)
	body, err := r.readTag(line)
	if err != nil {
		return err
	}
	return r.evaluate(body, line)
}

// readTag consumes the tag at r.pos, applies trimming, and returns its body after
// the opening marker, such as `= ENV["NAME"] ` for `<%= ENV["NAME"] %>`.
func (r *renderer) readTag(line int) (string, error) {
	start := r.pos + 2
	end := strings.Index(r.src[start:], "%>")
	if end < 0 {
		return "", fmt.Errorf("%s:%d: unterminated ERB tag: want a closing %%>", r.opts.File, line)
	}
	body := r.src[start : start+end]
	r.pos = start + end + 2
	if !r.opts.TrimMode {
		return body, nil
	}
	return r.trim(body), nil
}

// trim applies the <%- and -%> markers of Ruby's explicit trim mode. Ruby treats
// <%- as a statement tag, so <%-= is a statement that starts with "=". The leading
// space keeps evaluate from reading it as an output tag.
func (r *renderer) trim(body string) string {
	if strings.HasPrefix(body, "-") {
		body = " " + body[1:]
		r.trimIndent()
	}
	if strings.HasSuffix(body, "-") {
		body = body[:len(body)-1]
		r.skipNewline()
	}
	return body
}

// trimIndent removes the spaces and tabs before a <%- tag when only they precede it
// on its line.
func (r *renderer) trimIndent() {
	text := r.out.String()
	lineStart := strings.LastIndex(text, "\n") + 1
	if strings.Trim(text[lineStart:], " \t") != "" {
		return
	}
	r.out.Reset()
	r.out.WriteString(text[:lineStart])
}

// skipNewline drops the line break after a -%> tag.
func (r *renderer) skipNewline() {
	rest := r.src[r.pos:]
	switch {
	case strings.HasPrefix(rest, "\r\n"):
		r.pos += 2
	case strings.HasPrefix(rest, "\n"):
		r.pos++
	}
}

func (r *renderer) lineAt(offset int) int {
	return strings.Count(r.src[:offset], "\n") + 1
}

// evaluate writes the result of one tag body.
func (r *renderer) evaluate(body string, line int) error {
	switch {
	case strings.HasPrefix(body, "#"):
		return nil
	case strings.HasPrefix(body, "="):
		return r.output(strings.TrimSpace(body[1:]), line)
	default:
		return r.statement(strings.TrimSpace(body), line)
	}
}

// statement accepts a statement tag that holds no Ruby code, such as <% %> or
// <%- # note -%>, and rejects any other.
func (r *renderer) statement(code string, line int) error {
	if code == "" || strings.HasPrefix(code, "#") {
		return nil
	}
	return r.unsupported(code, line)
}

func (r *renderer) output(expr string, line int) error {
	value, err := evalExpression(expr, r.opts.Lookup)
	if err == errUnsupported {
		return r.unsupported(expr, line)
	}
	if err != nil {
		return fmt.Errorf("%s:%d: %w", r.opts.File, line, err)
	}
	r.out.WriteString(value)
	return nil
}

func (r *renderer) unsupported(expr string, line int) error {
	return fmt.Errorf("%s:%d: unsupported ERB expression: %s\n"+
		"  Supported: ENV[\"NAME\"], ENV.fetch(\"NAME\"), ENV.fetch(\"NAME\", \"default\"), \"text\"\n"+
		"  Move the value into an environment variable and read it with ENV[\"NAME\"].",
		r.opts.File, line, expr)
}
