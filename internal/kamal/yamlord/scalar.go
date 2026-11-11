package yamlord

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// The patterns below copy Psych::ScalarScanner from Ruby 3.4 (psych 5), with
// strict_integer false as YAML.unsafe_load uses it.
var (
	stringish     = regexp.MustCompile(`^[^\d.:-]?[[:alpha:]_\s!@#$%^&*(){}<>|/\\~;=]+`)
	notBoolStart  = regexp.MustCompile(`(?i)^[^ytonf~]`)
	nullWord      = regexp.MustCompile(`(?i)^null$`)
	trueWord      = regexp.MustCompile(`(?i)^(yes|true|on)$`)
	falseWord     = regexp.MustCompile(`(?i)^(no|false|off)$`)
	timeText      = regexp.MustCompile(`^-?\d{4}-\d{1,2}-\d{1,2}(?:[Tt]|\s+)\d{1,2}:\d\d:\d\d(?:\.\d*)?(?:\s*(?:Z|[-+]\d{1,2}:?(?:\d\d)?))?$`)
	dateText      = regexp.MustCompile(`^\d{4}-(?:1[012]|0\d|\d)-(?:[12]\d|3[01]|0\d|\d)$`)
	positiveInf   = regexp.MustCompile(`(?i)^\+?\.inf$`)
	negativeInf   = regexp.MustCompile(`(?i)^-\.inf$`)
	notANumber    = regexp.MustCompile(`(?i)^\.nan$`)
	quotedSymbol  = regexp.MustCompile(`^:(["'])(.*)(["'])`)
	base60Int     = regexp.MustCompile(`^[-+]?[0-9][0-9_]*(:[0-5]?[0-9]){1,2}$`)
	base60Float   = regexp.MustCompile(`^[-+]?[0-9][0-9_]*(:[0-5]?[0-9]){1,2}\.[0-9_]*$`)
	floatText     = regexp.MustCompile(`^(?:[-+]?([0-9][0-9_,]*)?\.[0-9]*([eE][-+][0-9]+)?)$`)
	loneDot       = regexp.MustCompile(`^[-+]?\.$`)
	dotBeforeExp  = regexp.MustCompile(`\.([Ee]|$)`)
	legacyInteger = regexp.MustCompile(`^(?:[-+]?0b[_,]*[0-1][0-1_,]*|[-+]?0[_,]*[0-7][0-7_,]*|[-+]?(?:0|[1-9](?:[0-9]|,[0-9]|_[0-9])*)|[-+]?0x[_,]*[0-9a-fA-F][0-9a-fA-F_,]*)$`)
)

// tokenize converts a plain (unquoted, untagged) scalar as Psych's
// ScalarScanner#tokenize does.
func tokenize(text string) *Value {
	if text == "" {
		return NewNull()
	}
	if stringish.MatchString(text) || strings.Contains(text, "\n") {
		return tokenizeWord(text)
	}
	return tokenizeNumberLike(text)
}

// tokenizeWord handles text that starts like a word: null, booleans, and strings.
func tokenizeWord(text string) *Value {
	switch {
	case len(text) > 5 || notBoolStart.MatchString(text):
		return NewString(text)
	case text == "~" || nullWord.MatchString(text):
		return NewNull()
	case trueWord.MatchString(text):
		return NewBool(true)
	case falseWord.MatchString(text):
		return NewBool(false)
	default:
		return NewString(text)
	}
}

func tokenizeNumberLike(text string) *Value {
	switch {
	case timeText.MatchString(text) || dateText.MatchString(text):
		return NewDate(text)
	case positiveInf.MatchString(text):
		return NewFloat(math.Inf(1))
	case negativeInf.MatchString(text):
		return NewFloat(math.Inf(-1))
	case notANumber.MatchString(text):
		return NewFloat(math.NaN())
	case strings.HasPrefix(text, ":") && len(text) > 1:
		return symbol(text)
	}
	return tokenizeNumber(text)
}

func symbol(text string) *Value {
	if m := quotedSymbol.FindStringSubmatch(text); m != nil && m[1] == m[3] {
		return NewSymbol(strings.TrimPrefix(m[2], ":"))
	}
	return NewSymbol(strings.TrimPrefix(text, ":"))
}

func tokenizeNumber(text string) *Value {
	switch {
	case base60Int.MatchString(text):
		return NewInt(int64(sexagesimal(text)))
	case base60Float.MatchString(text):
		return NewFloat(sexagesimal(text))
	case floatText.MatchString(text) && !loneDot.MatchString(text):
		return parseFloat(text)
	case legacyInteger.MatchString(text):
		return parseInt(text)
	default:
		return NewString(text)
	}
}

// sexagesimal follows Psych: each part n at index e adds n * 60 ** |e - 2|, so
// 3000:30 is 3000 * 3600 + 30 * 60.
func sexagesimal(text string) float64 {
	total := 0.0
	for e, part := range strings.Split(text, ":") {
		n, _ := strconv.ParseFloat(strings.ReplaceAll(part, "_", ""), 64)
		total += n * math.Pow(60, math.Abs(float64(e-2)))
	}
	return total
}

func parseFloat(text string) *Value {
	cleaned := strings.NewReplacer(",", "", "_", "").Replace(text)
	cleaned = dotBeforeExp.ReplaceAllString(cleaned, "$1")
	f, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return NewString(text)
	}
	return NewFloat(f)
}

// parseInt follows Ruby's Integer(): 0b binary, 0x hex, and a leading 0 for octal.
func parseInt(text string) *Value {
	cleaned := strings.NewReplacer(",", "", "_", "").Replace(text)
	sign, digits := splitSign(cleaned)
	base := 10
	switch {
	case strings.HasPrefix(digits, "0b"):
		base, digits = 2, digits[2:]
	case strings.HasPrefix(digits, "0x"):
		base, digits = 16, digits[2:]
	case len(digits) > 1 && digits[0] == '0':
		base, digits = 8, digits[1:]
	}
	n, err := strconv.ParseInt(sign+digits, base, 64)
	if err != nil {
		return NewString(text)
	}
	return NewInt(n)
}

func splitSign(text string) (string, string) {
	if strings.HasPrefix(text, "-") || strings.HasPrefix(text, "+") {
		return text[:1], text[1:]
	}
	return "", text
}
