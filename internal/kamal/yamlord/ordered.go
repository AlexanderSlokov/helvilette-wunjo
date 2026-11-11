package yamlord

// Ordered is a Hash with string keys that keeps insertion order, as Ruby's Hash does.
// Setting an existing key keeps the key in its first position.
//
//	m := yamlord.NewOrdered()
//	m.Set("service", yamlord.NewString("app"))
type Ordered struct {
	keys   []string
	values map[string]*Value
}

// NewOrdered returns an empty Hash.
func NewOrdered() *Ordered {
	return &Ordered{values: map[string]*Value{}}
}

// Pair is one key and value of a Hash literal.
type Pair struct {
	Key   string
	Value *Value
}

// OrderedOf builds a Hash from pairs in order, for example
// OrderedOf(Pair{"host", NewString("app.example.com")}).
func OrderedOf(pairs ...Pair) *Ordered {
	m := NewOrdered()
	for _, pair := range pairs {
		m.Set(pair.Key, pair.Value)
	}
	return m
}

// Len returns the number of keys. A nil Hash has none.
func (m *Ordered) Len() int {
	if m == nil {
		return 0
	}
	return len(m.keys)
}

// Keys returns the keys in insertion order.
func (m *Ordered) Keys() []string {
	if m == nil {
		return nil
	}
	return append([]string(nil), m.keys...)
}

// Get returns the value of key, as Ruby's hash[key], and whether the key exists.
func (m *Ordered) Get(key string) (*Value, bool) {
	if m == nil {
		return nil, false
	}
	value, ok := m.values[key]
	return value, ok
}

// At returns the value of key, or nil when the key is missing.
func (m *Ordered) At(key string) *Value {
	value, _ := m.Get(key)
	return value
}

// Has reports whether key exists, as Ruby's hash.key?(key).
func (m *Ordered) Has(key string) bool {
	_, ok := m.Get(key)
	return ok
}

// Set stores value under key.
func (m *Ordered) Set(key string, value *Value) {
	if _, ok := m.values[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

// Delete removes key, as Ruby's hash.delete(key).
func (m *Ordered) Delete(key string) {
	if _, ok := m.values[key]; !ok {
		return
	}
	delete(m.values, key)
	for i, k := range m.keys {
		if k == key {
			m.keys = append(m.keys[:i:i], m.keys[i+1:]...)
			return
		}
	}
}

// Clone returns a shallow copy.
func (m *Ordered) Clone() *Ordered {
	out := NewOrdered()
	for _, key := range m.Keys() {
		out.Set(key, m.values[key])
	}
	return out
}

// Except returns a copy without the given keys, as Ruby's hash.except(*keys).
func (m *Ordered) Except(keys ...string) *Ordered {
	out := m.Clone()
	for _, key := range keys {
		out.Delete(key)
	}
	return out
}

// Merge returns a copy with the keys of other added or replaced, as Ruby's
// hash.merge(other).
func (m *Ordered) Merge(other *Ordered) *Ordered {
	out := m.Clone()
	for _, key := range other.Keys() {
		out.Set(key, other.values[key])
	}
	return out
}

// DeepMerge returns a copy where nested Hashes merge key by key and other wins every
// other conflict, as ActiveSupport's hash.deep_merge(other).
func (m *Ordered) DeepMerge(other *Ordered) *Ordered {
	out := m.Clone()
	for _, key := range other.Keys() {
		out.Set(key, deepMergeValue(out.At(key), other.values[key]))
	}
	return out
}

func deepMergeValue(mine, theirs *Value) *Value {
	if mine.Kind() == Map && theirs.Kind() == Map {
		return NewMap(mine.mapping.DeepMerge(theirs.mapping))
	}
	return theirs
}

// Each calls visit for every key in insertion order.
func (m *Ordered) Each(visit func(key string, value *Value)) {
	for _, key := range m.Keys() {
		visit(key, m.values[key])
	}
}
