package configuration

// Ports lib/kamal/configuration/validator/*.rb at Kamal 2.12.0.

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// mainCommands lists Kamal::Cli::Main.commands at Kamal 2.12.0, which aliases may not
// shadow. Ruby 3.4 printed this list from the Kamal source.
var mainCommands = []string{
	"setup", "deploy", "redeploy", "rollback", "details", "audit", "config", "docs", "init",
	"remove", "upgrade", "version", "accessory", "app", "build", "lock", "proxy", "prune",
	"registry", "secrets", "server",
}

func validateAccessory(config, example *yamlord.Value, context string) error {
	v := validator{context: context}
	if err := v.againstExample(config, example); err != nil {
		return err
	}
	if countKeys(config.Map(), "host", "hosts", "role", "roles", "tag", "tags") != 1 {
		return v.fail("specify one of `host`, `hosts`, `role`, `roles`, `tag` or `tags`")
	}
	if err := v.labels(config.Map().At("labels")); err != nil {
		return err
	}
	return v.dockerOptions(config.Map().At("options"))
}

func countKeys(m *yamlord.Ordered, keys ...string) int {
	count := 0
	for _, key := range keys {
		if m.Has(key) {
			count++
		}
	}
	return count
}

var aliasName = regexp.MustCompile(`\A[a-z0-9_-]+\z`)

func validateAlias(command, example *yamlord.Value, context string) error {
	v := validator{context: context}
	if err := v.againstExample(command, example); err != nil {
		return err
	}
	name := strings.TrimPrefix(context, "aliases/")
	if !aliasName.MatchString(name) {
		return v.fail(fmt.Sprintf("Invalid alias name: '%s'. Must only contain lowercase letters, alphanumeric, hyphens and underscores.", name))
	}
	if utils.Contains(mainCommands, name) {
		return v.fail(fmt.Sprintf("Alias '%s' conflicts with a built-in command.", name))
	}
	return nil
}

func validateBuilder(config, example *yamlord.Value, context string) error {
	v := validator{context: context}
	if err := v.againstExample(config, example); err != nil {
		return err
	}
	return v.builderRules(config.Map())
}

func (v validator) builderRules(m *yamlord.Ordered) error {
	cacheType := m.At("cache").Dig("type")
	if m.At("cache").Truthy() && cacheType.Truthy() && cacheType.ToS() != "gha" && cacheType.ToS() != "registry" {
		return v.fail("Invalid cache type: " + cacheType.ToS())
	}
	if m.At("arch").Blank() {
		return v.fail("Builder arch not set")
	}
	if m.At("pack").Truthy() && len(m.At("arch").List()) > 1 {
		return v.fail("buildpacks only support building for one arch")
	}
	if m.At("local").IsFalse() && m.At("remote").Blank() {
		return v.fail("Cannot disable local builds, no remote is set")
	}
	return nil
}

var envSpecialKeys = []string{"clear", "secret", "tags"}

// validateEnv ports Validator::Env: a plain Hash of strings, or clear, secret, and
// tags (tags only in the root env).
func validateEnv(config, example *yamlord.Value, context string) error {
	v := validator{context: context}
	m := config.Map()
	if countKeys(m, envSpecialKeys...) == 0 {
		return v.hashOf(config, classString)
	}
	return v.complexEnv(m, example)
}

// complexEnv checks an env written with clear, secret, and tags keys.
func (v validator) complexEnv(m *yamlord.Ordered, example *yamlord.Value) error {
	if err := v.unknownKeysError(keysExcept(m, envSpecialKeys)); err != nil {
		return err
	}
	if m.Has("clear") {
		if err := v.in("clear").hashOf(m.At("clear"), classString); err != nil {
			return err
		}
	}
	if m.Has("secret") {
		if err := v.in("secret").arrayOf(m.At("secret"), classString); err != nil {
			return err
		}
	}
	if m.Has("tags") {
		return v.envTags(m.At("tags"), example)
	}
	return nil
}

func keysExcept(m *yamlord.Ordered, known []string) []string {
	var out []string
	for _, key := range m.Keys() {
		if !utils.Contains(known, key) {
			out = append(out, key)
		}
	}
	return out
}

func (v validator) envTags(tags, example *yamlord.Value) error {
	if v.context != "env" {
		return v.fail("tags are only allowed in the root env")
	}
	ctx := v.in("tags")
	if err := ctx.validateType(tags, classHash); err != nil {
		return err
	}
	tagExample := secondValue(example.Dig("tags"))
	for _, tag := range tags.Map().Keys() {
		if err := ctx.envTag(tag, tags.Map().At(tag), tagExample); err != nil {
			return err
		}
	}
	return nil
}

func (v validator) envTag(tag string, value, example *yamlord.Value) error {
	tagCtx := v.in(tag)
	if err := tagCtx.validateType(value, classHash); err != nil {
		return err
	}
	return validateEnv(value, example, tagCtx.context)
}

// secondValue returns example["tags"].values[1], the tag example with clear and
// secret keys.
func secondValue(tags *yamlord.Value) *yamlord.Value {
	keys := tags.Map().Keys()
	if len(keys) < 2 {
		return nil
	}
	return tags.Map().At(keys[1])
}

func validateProxy(config, example *yamlord.Value, context string) error {
	if config.IsNull() {
		return nil
	}
	v := validator{context: context}
	if err := v.againstExample(config, example); err != nil {
		return err
	}
	m := config.Map()
	if err := v.proxyHosts(m); err != nil {
		return err
	}
	if err := v.proxySSL(m.At("ssl")); err != nil {
		return err
	}
	return v.proxyRun(m)
}

func (v validator) proxyHosts(m *yamlord.Ordered) error {
	if m.At("host").Blank() && m.At("hosts").Blank() && m.At("ssl").Truthy() {
		return v.fail("Must set a host to enable automatic SSL")
	}
	if countKeys(m, "host", "hosts") > 1 {
		return v.fail("Specify one of 'host' or 'hosts', not both")
	}
	return nil
}

func (v validator) proxySSL(ssl *yamlord.Value) error {
	if ssl.Kind() != yamlord.Map {
		return nil
	}
	cert, key := ssl.Map().At("certificate_pem"), ssl.Map().At("private_key_pem")
	if cert.Present() && key.Blank() {
		return v.fail("Missing private_key_pem setting (required when certificate_pem is present)")
	}
	if key.Present() && cert.Blank() {
		return v.fail("Missing certificate_pem setting (required when private_key_pem is present)")
	}
	return nil
}

// proxyRun ports the run checks. Kamal checks config["bind_ips"], the proxy-level
// key, after it finds run.bind_ips; the port keeps that lookup.
func (v validator) proxyRun(m *yamlord.Ordered) error {
	run := m.At("run")
	if !run.Truthy() {
		return nil
	}
	if run.Dig("bind_ips").Present() {
		if err := v.bindIPs(m.At("bind_ips")); err != nil {
			return err
		}
	}
	if run.Dig("publish").IsFalse() && (run.Dig("bind_ips").Present() || run.Dig("http_port").Present() || run.Dig("https_port").Present()) {
		return v.fail("Cannot set http_port, https_port or bind_ips when publish is false")
	}
	return nil
}

func (v validator) bindIPs(ips *yamlord.Value) error {
	if ips.Blank() {
		return nil
	}
	for _, ip := range ips.List() {
		if !isIPAddress(ip.ToS()) {
			return v.fail("Invalid publish IP address: " + ip.ToS())
		}
	}
	return nil
}

// isIPAddress stands for Resolv::IPv4::Regex and Resolv::IPv6::Regex.
func isIPAddress(text string) bool { return net.ParseIP(text) != nil }

var localRegistry = regexp.MustCompile(`^localhost[:$]`)

// validateRegistry checks username and password as a String or a one-String Array
// (a secret name), except for a registry on localhost.
func validateRegistry(config, example *yamlord.Value, context string) error {
	v := validator{context: context}
	credentials := []string{"username", "password"}
	if err := v.againstExample(yamlord.NewMap(config.Map().Except(credentials...)), yamlord.NewMap(example.Map().Except(credentials...))); err != nil {
		return err
	}
	for _, key := range credentials {
		if err := v.in(key).credential(config.Map()); err != nil {
			return err
		}
	}
	return nil
}

func (v validator) credential(m *yamlord.Ordered) error {
	key := v.context[strings.LastIndex(v.context, "/")+1:]
	if localRegistry.MatchString(m.At("server").ToS()) && m.At("server").Kind() == yamlord.String {
		return nil
	}
	value := m.At(key)
	if value.Blank() {
		return v.fail("is required")
	}
	if !isCredentialShape(value) {
		return v.fail("should be a string or an array with one string (for secret lookup)")
	}
	return nil
}

func isCredentialShape(value *yamlord.Value) bool {
	if value.Kind() == yamlord.String {
		return true
	}
	items := value.List()
	return value.Kind() == yamlord.List && len(items) == 1 && items[0].Kind() == yamlord.String
}

func validateRole(config, example *yamlord.Value, context string) error {
	v := validator{context: context}
	if err := v.validateType(config, classArray, classHash); err != nil {
		return err
	}
	if config.Kind() == yamlord.List {
		return v.servers(config)
	}
	if err := v.againstExample(config, example); err != nil {
		return err
	}
	if err := v.labels(config.Map().At("labels")); err != nil {
		return err
	}
	return v.dockerOptions(config.Map().At("options"))
}

func validateServers(config *yamlord.Value, context string) error {
	v := validator{context: context}
	if err := v.validateType(config, classArray, classHash, classNil); err != nil {
		return err
	}
	if config.Kind() == yamlord.List {
		return v.servers(config)
	}
	return nil
}
