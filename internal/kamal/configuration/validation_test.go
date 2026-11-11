package configuration

// Ports test/configuration/validation_test.rb at Kamal 2.12.0.

import (
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/testsupport"
)

const validDeploy = `
service: app
image: app
builder: {arch: amd64}
registry: {username: user, password: secret}
servers: [1.1.1.1]
`

func assertValidationError(t *testing.T, want string, invalid string) {
	t.Helper()
	_, err := newWorld().build(merged(t, validDeploy, invalid))
	assertConfigurationError(t, err)
	expectError(t, err, want)
}

func TestUnknownRootKey(t *testing.T) {
	assertValidationError(t, "unknown key: unknown", "unknown: value")
	assertValidationError(t, "unknown keys: unknown, unknown2", "unknown: value\nunknown2: value")
}

func TestWrongRootTypes(t *testing.T) {
	for _, key := range []string{"service", "image", "asset_path", "hooks_path", "secrets_path", "primary_role", "minimum_version", "run_directory"} {
		assertValidationError(t, key+": should be a string", key+": []")
	}
	for _, key := range []string{"require_destination", "allow_empty_roles"} {
		assertValidationError(t, key+": should be a boolean", key+": foo")
	}
	for _, key := range []string{"deploy_timeout", "drain_timeout", "retain_containers", "readiness_delay"} {
		assertValidationError(t, key+": should be an integer", key+": foo")
	}
	assertValidationError(t, "volumes: should be an array", "volumes: foo")
	assertValidationError(t, "servers: should be an array or a hash", "servers: foo")
	for _, key := range []string{"labels", "registry", "accessories", "env", "ssh", "sshkit", "builder", "proxy", "boot", "logging"} {
		assertValidationError(t, key+": should be a hash", key+": []")
	}
}

func TestValidationServers(t *testing.T) {
	cases := [][2]string{
		{"servers: should be an array or a hash", "servers: foo"},
		{"servers/0: should be a string or a hash", "servers: [[]]"},
		{"servers/0: multiple hosts found", "servers: [{a: b, c: d}]"},
		{"servers/0/foo: should be a string or an array", "servers: [{foo: {}}]"},
		{"servers/0/foo/0: should be a string", "servers: [{foo: [[]]}]"},
	}
	for _, c := range cases {
		assertValidationError(t, c[0], c[1])
	}
}

func TestValidationRoles(t *testing.T) {
	cases := [][2]string{
		{"servers/web: should be an array or a hash", "servers: {web: foo}"},
		{"servers/web/hosts: should be an array", `servers: {web: {hosts: ""}}`},
		{"servers/web/hosts/0: should be a string or a hash", "servers: {web: {hosts: [[]]}}"},
		{"servers/web/options: should be a hash", `servers: {web: {options: ""}}`},
		{"servers/web/logging/options: should be a hash", `servers: {web: {logging: {options: ""}}}`},
		{"servers/web/logging/driver: should be a string", "servers: {web: {logging: {driver: []}}}"},
		{"servers/web/labels/service: invalid label. destination, role, and service are reserved labels", "servers: {web: {labels: {service: foo}}}"},
		{"servers/web/labels: should be a hash", "servers: {web: {labels: []}}"},
		{"servers/web/env: should be a hash", "servers: {web: {env: []}}"},
		{"servers/web/env: tags are only allowed in the root env", "servers: {web: {hosts: [1.1.1.1], env: {tags: {}}}}"},
	}
	for _, c := range cases {
		assertValidationError(t, c[0], c[1])
	}
}

func TestValidationRegistry(t *testing.T) {
	cases := [][2]string{
		{"registry/username: is required", "registry: {}"},
		{"registry/password: is required", "registry: {username: foo}"},
		{"registry/password: should be a string or an array with one string (for secret lookup)", "registry: {username: foo, password: [SECRET1, SECRET2]}"},
		{"registry/server: should be a string", "registry: {username: foo, password: bar, server: []}"},
	}
	for _, c := range cases {
		assertValidationError(t, c[0], c[1])
	}
}

func TestValidationAccessories(t *testing.T) {
	cases := [][2]string{
		{"accessories/accessory1: should be a hash", "accessories: {accessory1: []}"},
		{"accessories/accessory1: unknown key: unknown", "accessories: {accessory1: {unknown: baz}}"},
		{"accessories/accessory1/options: should be a hash", "accessories: {accessory1: {options: []}}"},
		{"accessories/accessory1/labels/destination: invalid label. destination, role, and service are reserved labels", "accessories: {accessory1: {host: host, labels: {destination: foo}}}"},
		{"accessories/accessory1/host: should be a string", "accessories: {accessory1: {host: []}}"},
		{"accessories/accessory1/env: should be a hash", "accessories: {accessory1: {env: []}}"},
		{"accessories/accessory1/env: tags are only allowed in the root env", "accessories: {accessory1: {host: host, env: {tags: {}}}}"},
	}
	for _, c := range cases {
		assertValidationError(t, c[0], c[1])
	}
}

func TestValidationEnv(t *testing.T) {
	cases := [][2]string{
		{"env: should be a hash", "env: []"},
		{"env/FOO: should be a string", "env: {FOO: []}"},
		{"env/clear/FOO: should be a string", "env: {clear: {FOO: []}}"},
		{"env/secret: should be an array", "env: {secret: {FOO: []}}"},
		{"env/secret/0: should be a string", "env: {secret: [[]]}"},
		{"env/tags: should be a hash", "env: {tags: []}"},
		{"env/tags/tag1: should be a hash", "env: {tags: {tag1: foo}}"},
		{"env/tags/tag1/FOO: should be a string", "env: {tags: {tag1: {FOO: []}}}"},
		{"env/tags/tag1/clear/FOO: should be a string", "env: {tags: {tag1: {clear: {FOO: []}}}}"},
		{"env/tags/tag1/secret: should be an array", "env: {tags: {tag1: {secret: {}}}}"},
		{"env/tags/tag1/secret/0: should be a string", "env: {tags: {tag1: {secret: [[]]}}}"},
		{"env/tags/tag1: tags are only allowed in the root env", "env: {tags: {tag1: {tags: {}}}}"},
	}
	for _, c := range cases {
		assertValidationError(t, c[0], c[1])
	}
}

func TestValidationSSH(t *testing.T) {
	assertValidationError(t, "ssh: unknown key: foo", "ssh: {foo: bar}")
	assertValidationError(t, "ssh/user: should be a string", "ssh: {user: []}")
	assertValidationError(t, "ssh/config: should be a boolean or a string or an array", "ssh: {config: 1}")
}

func TestValidationSshkit(t *testing.T) {
	assertValidationError(t, "sshkit: unknown key: foo", "sshkit: {foo: bar}")
	assertValidationError(t, "sshkit/max_concurrent_starts: should be an integer", "sshkit: {max_concurrent_starts: foo}")
	assertValidationError(t, "sshkit/dns_retries: should be an integer", "sshkit: {dns_retries: foo}")
}

func TestValidationBuilder(t *testing.T) {
	cases := [][2]string{
		{"builder: unknown key: foo", "builder: {foo: bar}"},
		{"builder/remote: should be a string", "builder: {remote: {foo: bar}}"},
		{"builder/arch: should be an array or a string", "builder: {arch: {}}"},
		{"builder/args: should be a hash", "builder: {args: [foo]}"},
		{"builder/cache/options: should be a string", "builder: {cache: {options: []}}"},
		{"builder: buildpacks only support building for one arch", "builder: {arch: [amd64, arm64], pack: {builder: 'heroku/builder:24'}}"},
	}
	for _, c := range cases {
		assertValidationError(t, c[0], c[1])
	}
}

func TestLocalRegistryWithRemoteBuilderRequiresSSHURL(t *testing.T) {
	arch := testsupport.RemoteArch
	assertValidationError(t, "Local registry with remote builder requires an SSH URL (e.g., ssh://user@host)",
		"registry: {server: 'localhost:5000'}\nbuilder: {arch: "+arch+", remote: 'docker-container://remote-builder'}")
	mustBuild(t, newWorld(), parseYAML(t, "service: app\nimage: app\nregistry: {server: 'localhost:5000'}\nbuilder: {arch: "+arch+", remote: 'ssh://user@host'}\nservers: [1.1.1.1]\n"))
}
