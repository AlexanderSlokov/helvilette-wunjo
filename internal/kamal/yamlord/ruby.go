package yamlord

import (
	"math"
	"strconv"
	"strings"
)

// Present reports Ruby's present?: nil, false, a blank String, and an empty Array or
// Hash are blank, and every other value is present.
func (v *Value) Present() bool {
	switch v.Kind() {
	case Null:
		return false
	case Bool:
		return v.boolean
	case String, Symbol:
		return strings.TrimSpace(v.text) != ""
	case List:
		return len(v.list) > 0
	case Map:
		return v.mapping.Len() > 0
	default:
		return true
	}
}

// Blank reports Ruby's blank?, the opposite of Present.
func (v *Value) Blank() bool { return !v.Present() }

// ToS returns Ruby's to_s for scalars: "" for nil, "true", "80", "1.5", or the text.
// An Array or Hash returns its Inspect form, as Ruby does.
func (v *Value) ToS() string {
	switch v.Kind() {
	case Null:
		return ""
	case Bool:
		return strconv.FormatBool(v.boolean)
	case Int:
		return strconv.FormatInt(v.integer, 10)
	case Float:
		return rubyFloat(v.float)
	case List, Map:
		return v.Inspect()
	default:
		return v.text
	}
}

// rubyFloat formats a float as Ruby's Float#to_s does: 1.0, 1.5, 1.0e+17, 1.5e-07.
func rubyFloat(f float64) string {
	switch {
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case math.IsNaN(f):
		return "NaN"
	}
	abs := math.Abs(f)
	if abs != 0 && (abs >= 1e16 || abs < 1e-4) {
		return rubyExponent(f)
	}
	text := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(text, ".") {
		text += ".0"
	}
	return text
}

// rubyExponent writes 1.0e+17 where Go writes 1e+17, and keeps two exponent digits.
func rubyExponent(f float64) string {
	text := strconv.FormatFloat(f, 'e', -1, 64)
	mantissa, exponent, _ := strings.Cut(text, "e")
	if !strings.Contains(mantissa, ".") {
		mantissa += ".0"
	}
	sign, digits := exponent[:1], exponent[1:]
	if len(digits) < 2 {
		digits = "0" + digits
	}
	return mantissa + "e" + sign + digits
}

// Inspect returns Ruby's inspect form, used in error messages:
// "web", nil, ["a", "b"], {"k" => "v"}.
func (v *Value) Inspect() string {
	switch v.Kind() {
	case Null:
		return "nil"
	case String, Date:
		return strconv.Quote(v.text)
	case Symbol:
		return ":" + v.text
	case List:
		return inspectList(v.list)
	case Map:
		return inspectMap(v.mapping)
	default:
		return v.ToS()
	}
}

func inspectList(items []*Value) string {
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = item.Inspect()
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func inspectMap(m *Ordered) string {
	parts := make([]string, 0, m.Len())
	m.Each(func(key string, value *Value) {
		parts = append(parts, strconv.Quote(key)+" => "+value.Inspect())
	})
	return "{" + strings.Join(parts, ", ") + "}"
}

// Equal reports Ruby's ==: same kind and same contents, where Hash order does not
// matter and 1 == 1.0.
func Equal(a, b *Value) bool {
	if isNumber(a) && isNumber(b) {
		return a.Float() == b.Float()
	}
	if a.Kind() != b.Kind() {
		return false
	}
	switch a.Kind() {
	case List:
		return equalLists(a.list, b.list)
	case Map:
		return equalMaps(a.mapping, b.mapping)
	default:
		return a.boolean == b.boolean && a.text == b.text
	}
}

func isNumber(v *Value) bool { return v.Kind() == Int || v.Kind() == Float }

func equalLists(a, b []*Value) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}

func equalMaps(a, b *Ordered) bool {
	if a.Len() != b.Len() {
		return false
	}
	for _, key := range a.Keys() {
		other, ok := b.Get(key)
		if !ok || !Equal(a.At(key), other) {
			return false
		}
	}
	return true
}

// Array returns Ruby's Array(value): [] for nil, the items of an Array, and a
// one-item list for any other value. Array of a Hash returns its pairs in Ruby; no
// Kamal caller depends on that, so the port returns the Hash as one item.
func (v *Value) Array() []*Value {
	switch v.Kind() {
	case Null:
		return nil
	case List:
		return v.list
	default:
		return []*Value{v}
	}
}

// StringList returns the ToS of each item of Array(value).
func (v *Value) StringList() []string {
	items := v.Array()
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.ToS()
	}
	return out
}

// Dig follows keys through nested Hashes, as Ruby's hash.dig(*keys), and returns nil
// when a key is missing or a value on the way is not a Hash.
func (v *Value) Dig(keys ...string) *Value {
	current := v
	for _, key := range keys {
		if current.Kind() != Map {
			return nil
		}
		current = current.mapping.At(key)
	}
	return current
}

// Fetch returns the value of key, or fallback when the key is missing, as Ruby's
// hash.fetch(key, fallback). A key that holds nil returns nil.
func (m *Ordered) Fetch(key string, fallback *Value) *Value {
	if value, ok := m.Get(key); ok {
		return value
	}
	return fallback
}
