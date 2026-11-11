// Package yamlord reads YAML into ordered values that behave like the Ruby objects
// that Psych returns. Kamal emits --env and --label flags in the order of the keys
// in deploy.yml, and Psych follows YAML 1.1, where `yes` is true and `3000:30` is a
// base-60 integer. Go maps have no order and gopkg.in/yaml.v3 follows YAML 1.2, so
// the port reads YAML through this package.
//
// Ports the parts of Psych and of Ruby's Hash, Array, and Object#present? that
// lib/kamal/configuration/ relies on at Kamal 2.12.0.
package yamlord

// Kind names the Ruby class that a value stands for.
type Kind int

const (
	// Null is Ruby's nil.
	Null Kind = iota
	// Bool is true or false.
	Bool
	// Int is an Integer.
	Int
	// Float is a Float.
	Float
	// String is a String.
	String
	// Symbol is a Symbol, written `:quiet` in YAML.
	Symbol
	// Date is a Date or Time. The port keeps its YAML text.
	Date
	// List is an Array.
	List
	// Map is a Hash with insertion order.
	Map
)

// Value is one YAML value. The zero Value is nil.
type Value struct {
	kind    Kind
	boolean bool
	integer int64
	float   float64
	text    string
	list    []*Value
	mapping *Ordered
}

// Kind returns the Ruby class of the value. A nil *Value is Null.
func (v *Value) Kind() Kind {
	if v == nil {
		return Null
	}
	return v.kind
}

// NewNull returns nil.
func NewNull() *Value { return &Value{kind: Null} }

// NewBool returns true or false, for example NewBool(true).
func NewBool(b bool) *Value { return &Value{kind: Bool, boolean: b} }

// NewInt returns an Integer, for example NewInt(80).
func NewInt(i int64) *Value { return &Value{kind: Int, integer: i} }

// NewFloat returns a Float, for example NewFloat(1.5).
func NewFloat(f float64) *Value { return &Value{kind: Float, float: f} }

// NewString returns a String, for example NewString("web").
func NewString(s string) *Value { return &Value{kind: String, text: s} }

// NewSymbol returns a Symbol without its colon, for example NewSymbol("quiet").
func NewSymbol(s string) *Value { return &Value{kind: Symbol, text: s} }

// NewDate returns a Date or Time that keeps its YAML text, for example
// NewDate("2024-01-02").
func NewDate(text string) *Value { return &Value{kind: Date, text: text} }

// NewList returns an Array of the given values.
func NewList(items ...*Value) *Value { return &Value{kind: List, list: items} }

// NewMap returns a Hash that holds m. A nil m makes an empty Hash.
func NewMap(m *Ordered) *Value {
	if m == nil {
		m = NewOrdered()
	}
	return &Value{kind: Map, mapping: m}
}

// Strings returns an Array of Strings, for example Strings("web-1", "web-2").
func Strings(items ...string) *Value {
	values := make([]*Value, len(items))
	for i, item := range items {
		values[i] = NewString(item)
	}
	return NewList(values...)
}

// Bool returns the boolean of a Bool value and false for any other kind.
func (v *Value) Bool() bool { return v.Kind() == Bool && v.boolean }

// Int returns the integer of an Int value and 0 for any other kind.
func (v *Value) Int() int64 {
	if v.Kind() != Int {
		return 0
	}
	return v.integer
}

// Float returns the number of an Int or Float value and 0 for any other kind.
func (v *Value) Float() float64 {
	switch v.Kind() {
	case Float:
		return v.float
	case Int:
		return float64(v.integer)
	default:
		return 0
	}
}

// Text returns the text of a String, Symbol, or Date, and "" for any other kind.
// Use ToS for Ruby's to_s.
func (v *Value) Text() string {
	switch v.Kind() {
	case String, Symbol, Date:
		return v.text
	default:
		return ""
	}
}

// List returns the items of an Array and nil for any other kind.
func (v *Value) List() []*Value {
	if v.Kind() != List {
		return nil
	}
	return v.list
}

// Map returns the Hash of a Map value and nil for any other kind.
func (v *Value) Map() *Ordered {
	if v.Kind() != Map {
		return nil
	}
	return v.mapping
}

// IsNull reports whether the value is nil.
func (v *Value) IsNull() bool { return v.Kind() == Null }

// IsFalse reports whether the value is the boolean false, as Ruby's `== false`.
func (v *Value) IsFalse() bool { return v.Kind() == Bool && !v.boolean }

// IsTrue reports whether the value is the boolean true, as Ruby's `== true`.
func (v *Value) IsTrue() bool { return v.Kind() == Bool && v.boolean }

// Truthy reports Ruby truthiness: every value except nil and false.
func (v *Value) Truthy() bool { return !v.IsNull() && !v.IsFalse() }
