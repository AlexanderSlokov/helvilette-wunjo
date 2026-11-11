package configuration

// Ports test/configuration/builder_test.rb at Kamal 2.12.0. The fake Docker reports
// amd64 as the local arch.

import "testing"

const builderDeploy = `
service: app
image: dhh/app
registry: {username: dhh, password: secret}
builder: {arch: amd64}
servers: [1.1.1.1]
`

func builderOf(t *testing.T, w *fakeWorld, builder string) *Builder {
	t.Helper()
	return mustBuild(t, w, merged(t, builderDeploy, builder)).Builder()
}

func TestBuilderDefaults(t *testing.T) {
	b := builderOf(t, newWorld(), "")
	if !b.IsLocal() || b.IsRemote() || b.Pack() || b.Remote() != "" || b.Cached() || b.CacheFrom() != "" || b.CacheTo() != "" {
		t.Fatal("wrong builder defaults")
	}
	if b.Args().Len() != 0 || b.Dockerfile() != "Dockerfile" || b.Context() != "." || b.SSH() != "" || !b.Provenance().IsNull() || !b.Sbom().IsNull() {
		t.Fatal("wrong builder input defaults")
	}
	if s, err := b.Secrets(); err != nil || s.Len() != 0 {
		t.Fatal("unexpected builder secrets")
	}
}

func TestBuilderPack(t *testing.T) {
	if !builderOf(t, newWorld(), "builder: {arch: arm64, pack: {builder: 'heroku/builder:24'}}").Pack() {
		t.Fatal("pack not detected")
	}
	b := builderOf(t, newWorld(), "builder: {arch: amd64, pack: {builder: 'heroku/builder:24', buildpacks: [heroku/ruby, heroku/procfile]}}")
	if b.PackBuilder() != "heroku/builder:24" || joined(b.PackBuildpacks()) != "heroku/ruby heroku/procfile" {
		t.Fatal("wrong pack details")
	}
}

func TestSettingBothLocalAndRemoteConfigs(t *testing.T) {
	b := builderOf(t, newWorld(), "builder: {arch: [amd64, arm64], remote: 'ssh://root@192.168.0.1'}")
	if !b.IsLocal() || !b.IsRemote() || joined(b.Arches()) != "amd64 arm64" || b.Remote() != "ssh://root@192.168.0.1" {
		t.Fatal("wrong hybrid builder")
	}
}

func TestInvalidCacheType(t *testing.T) {
	_, err := newWorld().build(merged(t, builderDeploy, "builder: {arch: amd64, cache: {type: invalid}}"))
	assertConfigurationError(t, err)
}

func TestBuilderCaches(t *testing.T) {
	cases := []struct {
		deploy, from, to string
	}{
		{"builder: {arch: amd64, cache: {type: gha, options: 'mode=max,scope=test'}}", "type=gha,scope=test", "type=gha,mode=max,scope=test"},
		{"builder: {arch: amd64, cache: {type: registry, options: 'mode=max,image-manifest=true,oci-mediatypes=true'}}", "type=registry,ref=dhh/app-build-cache", "type=registry,ref=dhh/app-build-cache,mode=max,image-manifest=true,oci-mediatypes=true"},
		{"builder: {arch: amd64, cache: {type: registry, options: 'mode=max,image-manifest=true,oci-mediatypes=true'}}\nregistry: {username: dhh, password: secret, server: registry.example.com}", "type=registry,ref=registry.example.com/dhh/app-build-cache", "type=registry,ref=registry.example.com/dhh/app-build-cache,mode=max,image-manifest=true,oci-mediatypes=true"},
		{"builder: {arch: amd64, cache: {type: registry, image: kamal, options: mode=max}}", "type=registry,ref=kamal", "type=registry,ref=kamal,mode=max"},
	}
	for _, c := range cases {
		b := builderOf(t, newWorld(), c.deploy)
		if b.CacheFrom() != c.from || b.CacheTo() != c.to {
			t.Errorf("%s:\nfrom %s\nto   %s", c.deploy, b.CacheFrom(), b.CacheTo())
		}
	}
}

func TestBuilderSettings(t *testing.T) {
	b := builderOf(t, newWorld().withSecrets("GITHUB_TOKEN=secret123"), `builder:
  arch: amd64
  args: {key: value}
  secrets: [GITHUB_TOKEN]
  dockerfile: Dockerfile.dev
  context: ".."
  ssh: default=$SSH_AUTH_SOCK
  provenance: mode=max
  sbom: true
`)
	s, err := b.Secrets()
	if err != nil || s.At("GITHUB_TOKEN").ToS() != "secret123" || b.Args().At("key").ToS() != "value" {
		t.Fatalf("args/secrets wrong: %v", err)
	}
	if b.Dockerfile() != "Dockerfile.dev" || b.Context() != ".." || b.SSH() != "default=$SSH_AUTH_SOCK" || b.Provenance().ToS() != "mode=max" || !b.Sbom().IsTrue() {
		t.Fatal("builder settings not read")
	}
}

func TestLocalDisabledButNoRemoteSet(t *testing.T) {
	_, err := newWorld().build(merged(t, builderDeploy, "builder: {arch: amd64, local: false}"))
	assertConfigurationError(t, err)
}

func TestLocalDisabledAllArchesAreRemote(t *testing.T) {
	b := builderOf(t, newWorld(), "builder: {arch: [amd64, arm64], local: false, remote: 'ssh://root@192.168.0.1'}")
	if len(b.LocalArches()) != 0 || joined(b.RemoteArches()) != "amd64 arm64" {
		t.Fatalf("local %v remote %v", b.LocalArches(), b.RemoteArches())
	}
}
