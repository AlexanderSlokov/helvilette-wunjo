package configuration

// Ports lib/kamal/configuration/validator.rb at Kamal 2.12.0.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/kamalerr"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// rubyClass names the Ruby class that the validator compares, because Kamal checks
// a value against the class of the example value in the configuration documents.
type rubyClass string

const (
	classString  rubyClass = "String"
	classSymbol  rubyClass = "Symbol"
	classInteger rubyClass = "Integer"
	classFloat   rubyClass = "Float"
	classTrue    rubyClass = "TrueClass"
	classFalse   rubyClass = "FalseClass"
	classNil     rubyClass = "NilClass"
	classArray   rubyClass = "Array"
	classHash    rubyClass = "Hash"
	classDate    rubyClass = "Date"
)

var classOfKind = map[yamlord.Kind]rubyClass{
	yamlord.String: classString, yamlord.Symbol: classSymbol, yamlord.Int: classInteger,
	yamlord.Float: classFloat, yamlord.List: classArray, yamlord.Map: classHash,
	yamlord.Date: classDate, yamlord.Null: classNil,
}

func classOf(v *yamlord.Value) rubyClass {
	if v.Kind() == yamlord.Bool {
		return booleanClass(v)
	}
	return classOfKind[v.Kind()]
}

func booleanClass(v *yamlord.Value) rubyClass {
	if v.Bool() {
		return classTrue
	}
	return classFalse
}

func isBooleanClass(c rubyClass) bool { return c == classTrue || c == classFalse }

// stringish follows Validator#stringish?: Strings, Symbols, numbers, and booleans
// pass where the example holds a String.
func stringish(c rubyClass) bool {
	return c == classString || c == classSymbol || c == classInteger || c == classFloat || isBooleanClass(c)
}

func validType(value *yamlord.Value, want rubyClass) bool {
	got := classOf(value)
	return got == want || (want == classString && stringish(got)) || (isBooleanClass(want) && isBooleanClass(got))
}

func typeDescription(c rubyClass) string {
	switch {
	case c == classInteger || c == classArray:
		return "an " + strings.ToLower(string(c))
	case isBooleanClass(c):
		return "a boolean"
	default:
		return "a " + strings.ToLower(string(c))
	}
}

// validator walks a configuration section next to its example from the
// configuration documents. Its context is the slash-separated path, such as
// "servers/web/env", that prefixes every error.
type validator struct {
	context         string
	allowExtensions bool
	symbolKeys      bool
}

func (v validator) in(part string) validator {
	child := v
	child.symbolKeys = false
	if strings.TrimSpace(v.context) == "" {
		child.context = part
	} else if strings.TrimSpace(part) != "" {
		child.context = v.context + "/" + part
	}
	return child
}

func (v validator) at(index int) validator { return v.in(strconv.Itoa(index)) }

func (v validator) fail(message string) error {
	if strings.TrimSpace(v.context) != "" {
		return kamalerr.Configuration(v.context + ": " + message)
	}
	return kamalerr.Configuration(message)
}

func (v validator) typeError(classes ...rubyClass) error {
	var descriptions []string
	for _, c := range classes {
		if d := typeDescription(c); !utils.Contains(descriptions, d) {
			descriptions = append(descriptions, d)
		}
	}
	return v.fail("should be " + strings.Join(descriptions, " or "))
}

func (v validator) validateType(value *yamlord.Value, classes ...rubyClass) error {
	for _, c := range classes {
		if validType(value, c) {
			return nil
		}
	}
	return v.typeError(classes...)
}

// againstExample ports validate_against_example!.
func (v validator) againstExample(config, example *yamlord.Value) error {
	if err := v.validateType(config, classOf(example)); err != nil {
		return err
	}
	if example.Kind() != yamlord.Map {
		return nil
	}
	if err := v.checkUnknownKeys(config.Map(), example.Map()); err != nil {
		return err
	}
	return v.eachKnownKey(config.Map(), example.Map())
}

func (v validator) eachKnownKey(config, example *yamlord.Ordered) error {
	for _, key := range config.Keys() {
		if isExtension(key) {
			continue
		}
		if err := v.validateKey(key, config.At(key), example.At(key)); err != nil {
			return err
		}
	}
	return nil
}

func (v validator) validateKey(key string, value, example *yamlord.Value) error {
	child := v.in(key)
	switch {
	case example.Kind() == yamlord.String && example.Text() == "...":
		return child.sectionPlaceholder(v.symbolKeys, key, value)
	case key == "ssl":
		return child.validateType(value, classTrue, classFalse, classHash)
	case key == "hooks_output":
		return child.hooksOutput(value)
	case key == "hosts" && !v.symbolKeys:
		return child.servers(value)
	case example.Kind() == yamlord.List:
		return child.listKey(key, value, example, v.symbolKeys)
	case example.Kind() == yamlord.Map:
		return child.hashKey(key, value, example)
	default:
		return child.validateType(value, classOf(example))
	}
}

// sectionPlaceholder checks a key whose example is "...": a section validated by its
// own class later. Top-level servers may be an Array, and proxy may be a boolean.
func (v validator) sectionPlaceholder(topLevel bool, key string, value *yamlord.Value) error {
	if key == "proxy" && isBooleanClass(classOf(value)) {
		return nil
	}
	if topLevel && key == "servers" {
		return v.validateType(value, classArray, classHash)
	}
	return v.validateType(value, classHash)
}

func (v validator) listKey(key string, value, example *yamlord.Value, topLevel bool) error {
	first := classOf(example.List()[0])
	switch {
	case key == "arch" && !topLevel:
		return v.arrayOfOrType(value, first)
	case key == "config":
		return v.sshConfig(value)
	case key == "files" || key == "directories":
		return v.paths(value)
	default:
		return v.arrayOf(value, first)
	}
}

func (v validator) hashKey(key string, value, example *yamlord.Value) error {
	switch key {
	case "options", "args":
		return v.validateType(value, classHash)
	case "labels":
		firstValue := example.Map().At(example.Map().Keys()[0])
		return v.hashOf(value, classOf(firstValue))
	default:
		return v.againstExample(value, example)
	}
}

func (v validator) arrayOfOrType(value *yamlord.Value, c rubyClass) error {
	var err error
	if value.Kind() == yamlord.List {
		err = v.arrayOf(value, c)
	} else {
		err = v.validateType(value, c)
	}
	if err != nil {
		return v.typeError(classArray, c)
	}
	return nil
}

func (v validator) arrayOf(value *yamlord.Value, c rubyClass) error {
	if err := v.validateType(value, classArray); err != nil {
		return err
	}
	for i, item := range value.List() {
		if err := v.at(i).validateType(item, c); err != nil {
			return err
		}
	}
	return nil
}

func (v validator) hashOf(value *yamlord.Value, c rubyClass) error {
	if err := v.validateType(value, classHash); err != nil {
		return err
	}
	for _, key := range value.Map().Keys() {
		if err := v.in(key).validateType(value.Map().At(key), c); err != nil {
			return err
		}
	}
	return nil
}

// servers ports validate_servers!: each host is a String, or a one-key Hash from the
// host to its tags.
func (v validator) servers(value *yamlord.Value) error {
	if err := v.validateType(value, classArray); err != nil {
		return err
	}
	for i, server := range value.List() {
		if err := v.at(i).server(server); err != nil {
			return err
		}
	}
	return nil
}

func (v validator) server(server *yamlord.Value) error {
	if err := v.validateType(server, classString, classHash); err != nil {
		return err
	}
	if server.Kind() != yamlord.Map {
		return nil
	}
	if server.Map().Len() != 1 {
		return v.fail("multiple hosts found")
	}
	host := server.Map().Keys()[0]
	return v.in(host).hostTags(server.Map().At(host))
}

func (v validator) hostTags(tags *yamlord.Value) error {
	if err := v.validateType(tags, classString, classArray); err != nil {
		return err
	}
	if tags.Kind() == yamlord.List {
		return v.arrayOf(tags, classString)
	}
	return nil
}

func (v validator) sshConfig(value *yamlord.Value) error {
	c := classOf(value)
	switch {
	case c == classArray:
		return v.arrayOf(value, classString)
	case isBooleanClass(c) || c == classString:
		return nil
	default:
		return v.typeError(classTrue, classFalse, classString, classArray)
	}
}

func (v validator) paths(value *yamlord.Value) error {
	if err := v.validateType(value, classArray); err != nil {
		return err
	}
	for i, path := range value.List() {
		if err := v.at(i).path(path); err != nil {
			return err
		}
	}
	return nil
}

func (v validator) path(path *yamlord.Value) error {
	if err := v.validateType(path, classString, classHash); err != nil {
		return err
	}
	for _, key := range []string{"local", "remote", "mode", "owner", "options"} {
		if !path.Map().Has(key) {
			continue
		}
		if err := v.in(key).validateType(path.Map().At(key), classString); err != nil {
			return err
		}
	}
	return nil
}

func (v validator) hooksOutput(value *yamlord.Value) error {
	if value.Kind() != yamlord.Map {
		return v.validateType(value, classString, classSymbol)
	}
	for _, hook := range value.Map().Keys() {
		if err := v.in(hook).validateType(value.Map().At(hook), classString, classSymbol); err != nil {
			return err
		}
	}
	return nil
}

func isExtension(key string) bool { return strings.HasPrefix(key, "x-") }

func (v validator) checkUnknownKeys(config, example *yamlord.Ordered) error {
	var unknown []string
	for _, key := range config.Keys() {
		if !example.Has(key) && !(v.allowExtensions && isExtension(key)) {
			unknown = append(unknown, key)
		}
	}
	return v.unknownKeysError(unknown)
}

func (v validator) unknownKeysError(unknown []string) error {
	if len(unknown) == 0 {
		return nil
	}
	noun := "key"
	if len(unknown) > 1 {
		noun = "keys"
	}
	return v.fail(fmt.Sprintf("unknown %s: %s", noun, strings.Join(unknown, ", ")))
}

func (v validator) labels(labels *yamlord.Value) error {
	if labels.Blank() {
		return nil
	}
	for _, key := range labels.Map().Keys() {
		if key == "destination" || key == "role" || key == "service" {
			return v.in("labels").in(key).fail("invalid label. destination, role, and service are reserved labels")
		}
	}
	return nil
}

var restartPolicy = regexp.MustCompile(`\A(?:no|always|unless-stopped|on-failure(?::\d+)?)\z`)

func (v validator) dockerOptions(options *yamlord.Value) error {
	m := options.Map()
	for _, key := range m.Keys() {
		if key == "restart" {
			return v.restartPolicy(m.At(key))
		}
	}
	return nil
}

func (v validator) restartPolicy(policy *yamlord.Value) error {
	ctx := v.in("options/restart")
	if policy.Kind() != yamlord.String {
		return ctx.fail(`should be a string. Use "no" to disable restarts`)
	}
	if !restartPolicy.MatchString(policy.Text()) {
		return ctx.fail("should be no, always, unless-stopped, on-failure, or on-failure:N")
	}
	return nil
}
