package yamlord

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Parse reads one YAML document, as Ruby's YAML.unsafe_load does. An empty document,
// or one with only comments, returns false, as Psych does.
//
//	doc, err := yamlord.Parse([]byte("service: app\n"))
//	service := doc.Map().At("service").ToS()
func Parse(src []byte) (*Value, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	if len(root.Content) == 0 {
		return NewBool(false), nil
	}
	return convert(root.Content[0])
}

func convert(node *yaml.Node) (*Value, error) {
	switch node.Kind {
	case yaml.AliasNode:
		return convert(node.Alias)
	case yaml.SequenceNode:
		return convertSequence(node)
	case yaml.MappingNode:
		return convertMapping(node)
	case yaml.ScalarNode:
		return convertScalar(node), nil
	default:
		return nil, fmt.Errorf("line %d: unsupported YAML node kind %d: want a scalar, sequence, mapping, or alias", node.Line, node.Kind)
	}
}

func convertSequence(node *yaml.Node) (*Value, error) {
	items := make([]*Value, 0, len(node.Content))
	for _, child := range node.Content {
		item, err := convert(child)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return NewList(items...), nil
}

// convertMapping builds a Hash in key order. A merge key (<<) merges its Hash into
// the keys read so far, as Psych does, so keys after it win and keys before it lose.
func convertMapping(node *yaml.Node) (*Value, error) {
	out := NewOrdered()
	for i := 0; i+1 < len(node.Content); i += 2 {
		if err := addEntry(out, node.Content[i], node.Content[i+1]); err != nil {
			return nil, err
		}
	}
	return NewMap(out), nil
}

func addEntry(out *Ordered, keyNode, valueNode *yaml.Node) error {
	value, err := convert(valueNode)
	if err != nil {
		return err
	}
	if isMergeKey(keyNode) {
		return mergeInto(out, value, keyNode.Line)
	}
	key, err := convert(keyNode)
	if err != nil {
		return err
	}
	out.Set(key.ToS(), value)
	return nil
}

func isMergeKey(node *yaml.Node) bool {
	return node.Kind == yaml.ScalarNode && node.Value == "<<" && node.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle) == 0
}

func mergeInto(out *Ordered, value *Value, line int) error {
	sources := []*Value{value}
	if value.Kind() == List {
		sources = value.list
	}
	for _, source := range sources {
		if source.Kind() != Map {
			return fmt.Errorf("line %d: merge key << needs a mapping or a list of mappings, got %s", line, source.Inspect())
		}
		source.mapping.Each(out.Set)
	}
	return nil
}

// convertScalar applies Psych's rules: quoted and block scalars are strings, an
// explicit !!str or ! tag makes a string, and plain scalars go through tokenize.
func convertScalar(node *yaml.Node) *Value {
	if node.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle|yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return NewString(node.Value)
	}
	if node.Style&yaml.TaggedStyle != 0 {
		return taggedScalar(node)
	}
	return tokenize(node.Value)
}

func taggedScalar(node *yaml.Node) *Value {
	switch node.Tag {
	case "!!str", "!":
		return NewString(node.Value)
	case "!!null":
		return NewNull()
	default:
		return tokenize(node.Value)
	}
}
