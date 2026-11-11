package secrets

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// dotenvLine copies Dotenv::Parser::LINE from dotenv 3.2.0.
var dotenvLine = regexp.MustCompile(`(?m)(?:^|\A)\s*(?P<export>export\s+)?(?P<key>[\w.]+)(?:(?:\s*=\s*?|:\s+?)(?P<value>\s*'(?:\\'|[^'])*'|\s*"(?:\\"|[^"])*"|[^#\n]+)?)?\s*(?:#.*)?(?:$|\z)`)

var quotedString = regexp.MustCompile(`(?s)\A(['"])(.*)(['"])\z`)

// Substitution runs the $(...) commands and reads the $VAR references of a dotenv
// value.
type Substitution struct {
	// Shell runs $(command) substitutions.
	Shell localshell.Shell
	// Lookup reads the process environment for $VAR when the file has not set VAR.
	Lookup func(name string) (string, bool)
	// InlineSecrets runs `kamal secrets ...` commands in process, as Kamal does
	// instead of starting a second kamal. It receives the words after "kamal" with
	// "--inline" appended.
	InlineSecrets func(args []string) (string, error)
}

// ParseDotenv parses one secrets file as Dotenv::Parser.call(text, overwrite: true)
// does with Kamal's command substitution installed.
//
//	values, err := secrets.ParseDotenv("SECRET=$(op read op://vault/key)\n", sub)
func ParseDotenv(text string, sub Substitution) (*yamlord.Ordered, error) {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	p := &dotenvParser{sub: sub, values: yamlord.NewOrdered()}
	for _, m := range dotenvLine.FindAllStringSubmatchIndex(text, -1) {
		if err := p.line(text, m); err != nil {
			return nil, err
		}
	}
	return p.values, nil
}

type dotenvParser struct {
	sub    Substitution
	values *yamlord.Ordered
}

func (p *dotenvParser) line(text string, m []int) error {
	key := text[m[4]:m[5]]
	hasExport, hasValue := m[2] >= 0, m[6] >= 0
	if hasExport && !hasValue {
		if !p.values.Has(key) {
			return fmt.Errorf("Line %q has an unset variable", text[m[0]:m[1]])
		}
		return nil
	}
	raw := ""
	if hasValue {
		raw = text[m[6]:m[7]]
	}
	value, err := p.parseValue(raw)
	if err != nil {
		return err
	}
	p.values.Set(key, yamlord.NewString(value))
	return nil
}

// parseValue removes surrounding quotes, expands \n in double quotes, and runs the
// substitutions unless the value is single-quoted.
func (p *dotenvParser) parseValue(raw string) (string, error) {
	value, quote := unquote(strings.TrimSpace(raw))
	if quote == `"` {
		value = p.expandNewlines(value)
	}
	if quote == "'" {
		return value, nil
	}
	value = unescapeCharacters(value)
	value, err := p.substituteCommands(value)
	if err != nil {
		return "", err
	}
	return p.substituteVariables(value), nil
}

func unquote(value string) (string, string) {
	m := quotedString.FindStringSubmatch(value)
	if m == nil || m[1] != m[3] {
		return value, ""
	}
	return m[2], m[1]
}

// expandNewlines keeps \n and \r as an escaped backslash sequence, unless
// DOTENV_LINEBREAK_MODE is legacy, where they become real line breaks.
func (p *dotenvParser) expandNewlines(value string) string {
	if p.linebreakMode() == "legacy" {
		return strings.NewReplacer(`\n`, "\n", `\r`, "\r").Replace(value)
	}
	return strings.NewReplacer(`\n`, `\\n`, `\r`, `\\r`).Replace(value)
}

func (p *dotenvParser) linebreakMode() string {
	if mode, ok := p.values.Get("DOTENV_LINEBREAK_MODE"); ok {
		return mode.ToS()
	}
	mode, _ := p.lookupEnv("DOTENV_LINEBREAK_MODE")
	return mode
}

func (p *dotenvParser) lookupEnv(name string) (string, bool) {
	if p.sub.Lookup == nil {
		return "", false
	}
	return p.sub.Lookup(name)
}

// unescapeCharacters removes a backslash before any character except $, as the Ruby
// pattern /\\([^$])/ does.
func unescapeCharacters(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] == '\\' && i+1 < len(value) && value[i+1] != '$' {
			i++
		}
		out.WriteByte(value[i])
	}
	return out.String()
}

// InlineCommandSubstitution replaces the $(command) substitutions in value with an
// empty variable scope, as Kamal::Secrets::Dotenv::InlineCommandSubstitution.call
// does when dotenv calls it.
//
//	out, err := secrets.InlineCommandSubstitution("FOO=$(echo hi)", sub) // FOO=hi
func InlineCommandSubstitution(value string, sub Substitution) (string, error) {
	p := &dotenvParser{sub: sub, values: yamlord.NewOrdered()}
	return p.substituteCommands(value)
}

// substituteCommands replaces each $(command) with its output, as Kamal's
// InlineCommandSubstitution does. A backslash before $ keeps the text.
func (p *dotenvParser) substituteCommands(value string) (string, error) {
	var out strings.Builder
	for i := 0; i < len(value); {
		escaped := value[i] == '\\' && strings.HasPrefix(value[i+1:], "$(")
		dollar := i
		if escaped {
			dollar = i + 1
		}
		end := commandEnd(value, dollar)
		if end < 0 {
			out.WriteByte(value[i])
			i++
			continue
		}
		replacement, err := p.commandResult(value[dollar:end], escaped)
		if err != nil {
			return "", err
		}
		out.WriteString(replacement)
		i = end
	}
	return out.String(), nil
}

// commandEnd returns the end of a $(...) that starts at value[dollar], or -1.
func commandEnd(value string, dollar int) int {
	if !strings.HasPrefix(value[dollar:], "$(") {
		return -1
	}
	return balancedParens(value, dollar+1)
}

// balancedParens matches "(" items ")" from value[open], where an item is a
// backslash and any character except newline, a character other than ( ) \, or a
// nested group. It returns the index after the closing paren, or -1.
func balancedParens(value string, open int) int {
	i, items := open+1, 0
	for i < len(value) && value[i] != ')' {
		next := parenItemEnd(value, i)
		if next < 0 {
			return -1
		}
		i, items = next, items+1
	}
	if i >= len(value) || items == 0 {
		return -1
	}
	return i + 1
}

func parenItemEnd(value string, i int) int {
	switch value[i] {
	case '\\':
		if i+1 < len(value) && value[i+1] != '\n' {
			return i + 2
		}
		return -1
	case '(':
		return balancedParens(value, i)
	default:
		return i + 1
	}
}

var inlineSecretsCommand = regexp.MustCompile(`\A\s*kamal\s*secrets\s+`)

func (p *dotenvParser) commandResult(match string, escaped bool) (string, error) {
	if escaped {
		return match, nil
	}
	command := p.substituteVariables(match[2 : len(match)-1])
	if inlineSecretsCommand.MatchString(command) {
		return p.inlineSecrets(command)
	}
	return p.runCommand(command)
}

func (p *dotenvParser) inlineSecrets(command string) (string, error) {
	words, err := shell.Split(command)
	if err != nil {
		return "", err
	}
	if p.sub.InlineSecrets == nil {
		return "", fmt.Errorf("cannot run %q inline: no kamal secrets command is available", command)
	}
	out, err := p.sub.InlineSecrets(append(words[1:], "--inline"))
	return chomp(out), err
}

func (p *dotenvParser) runCommand(command string) (string, error) {
	result, err := p.sub.Shell.Run(command)
	if err != nil {
		return "", err
	}
	return chomp(result.Stdout), nil
}

// chomp removes one trailing line break, as Ruby's String#chomp does.
func chomp(text string) string {
	for _, ending := range []string{"\r\n", "\n", "\r"} {
		if strings.HasSuffix(text, ending) {
			return strings.TrimSuffix(text, ending)
		}
	}
	return text
}

// substituteVariables replaces $VAR and ${VAR} with the value set earlier in the same
// file, then the process environment, then "", as Dotenv::Substitutions::Variable
// does. A backslash before $ keeps the reference.
func (p *dotenvParser) substituteVariables(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); {
		ref, ok := variableAt(value, i)
		if !ok {
			out.WriteByte(value[i])
			i++
			continue
		}
		out.WriteString(p.variableValue(ref))
		i = ref.end
	}
	return out.String()
}

// variableRef is one match of the Ruby pattern (\\)?(\$)(?!\()\{?([A-Z0-9_]+)?\}?/i.
type variableRef struct {
	text    string
	escaped bool
	name    string
	end     int
}

func variableAt(value string, i int) (variableRef, bool) {
	start := i
	escaped := value[i] == '\\' && strings.HasPrefix(value[i+1:], "$") && !strings.HasPrefix(value[i+2:], "(")
	if escaped {
		i++
	}
	if value[i] != '$' || strings.HasPrefix(value[i+1:], "(") {
		return variableRef{}, false
	}
	i++
	if i < len(value) && value[i] == '{' {
		i++
	}
	nameStart := i
	for i < len(value) && isVariableChar(value[i]) {
		i++
	}
	name := value[nameStart:i]
	if i < len(value) && value[i] == '}' {
		i++
	}
	return variableRef{text: value[start:i], escaped: escaped, name: name, end: i}, true
}

func isVariableChar(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}

func (p *dotenvParser) variableValue(ref variableRef) string {
	switch {
	case ref.escaped:
		return ref.text[1:]
	case ref.name == "":
		return ref.text
	}
	// Ruby treats "" as truthy, so a variable set to "" in the file wins over the
	// process environment.
	if value, ok := p.values.Get(ref.name); ok {
		return value.ToS()
	}
	value, _ := p.lookupEnv(ref.name)
	return value
}
