package commander

// Ports test/commander_test.rb at Kamal 2.12.0.

import (
	"errors"
	"strings"
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/commands"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/git"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/testsupport"
)

func configureWith(t *testing.T, variant string) *Commander {
	t.Helper()
	files := testsupport.FakeFiles{}
	content, err := testsupport.ReadFixture(variant + ".yml")
	if err != nil {
		t.Fatal(err)
	}
	files["fixtures/"+variant+".yml"] = content
	env := testsupport.FakeEnv{"VERSION": "999", testsupport.RemoteArchVariable: testsupport.RemoteArch}
	sh := &localshell.Fake{Results: map[string]localshell.Result{"docker info --format '{{.Architecture}}'": {Stdout: "x86_64\n", Success: true}}}
	c := New(configuration.Environment{Lookup: env.Lookup, Shell: sh, Git: git.Git{Shell: sh}, Files: files, SecretFiles: files}, commands.Runtime{}, false)
	c.Configure(ConfigOptions{ConfigFile: "fixtures/" + variant + ".yml"})
	if _, err := c.Config(); err != nil {
		t.Fatal(err)
	}
	return c
}

func names(roles []*configuration.Role) string {
	var out []string
	for _, role := range roles {
		out = append(out, role.Name())
	}
	return strings.Join(out, " ")
}

func joined(items []string) string { return strings.Join(items, " ") }

func mustSet(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestLazyConfiguration(t *testing.T) {
	if configureWith(t, "deploy_with_roles").MustConfig() == nil {
		t.Fatal("no configuration")
	}
}

func TestOverwritingHosts(t *testing.T) {
	c := configureWith(t, "deploy_with_roles")
	all := "1.1.1.1 1.1.1.2 1.1.1.3 1.1.1.4"
	cases := []struct {
		patterns []string
		want     string
	}{
		{nil, all}, {[]string{"1.1.1.1", "1.1.1.2"}, "1.1.1.1 1.1.1.2"}, {[]string{"1.1.1.1*"}, "1.1.1.1"},
		{[]string{"1.1.1.*", "*.1.2.*"}, all}, {[]string{"*"}, all}, {[]string{"1.1.1.[12]"}, "1.1.1.1 1.1.1.2"},
	}
	for _, tc := range cases {
		mustSet(t, c.SetSpecificHosts(tc.patterns))
		if got := joined(c.Hosts()); got != tc.want {
			t.Errorf("%v: hosts = %s, want %s", tc.patterns, got, tc.want)
		}
	}
	if err := c.SetSpecificHosts([]string{"*miss"}); err == nil || !strings.Contains(err.Error(), "hosts match for *miss") {
		t.Fatalf("error = %v", err)
	}
}

func TestFilteringHostsByFilteringRoles(t *testing.T) {
	c := configureWith(t, "deploy_with_roles")
	mustSet(t, c.SetSpecificRoles([]string{"web"}))
	if joined(c.Hosts()) != "1.1.1.1 1.1.1.2" {
		t.Fatalf("hosts = %v", c.Hosts())
	}
	if err := c.SetSpecificRoles([]string{"*miss"}); err == nil || !strings.Contains(err.Error(), "roles match for *miss") {
		t.Fatalf("error = %v", err)
	}
}

func TestFilteringRoles(t *testing.T) {
	c := configureWith(t, "deploy_with_roles")
	cases := []struct {
		patterns []string
		want     string
	}{
		{nil, "web workers"}, {[]string{"workers"}, "workers"}, {[]string{"w*"}, "web workers"},
		{[]string{"we*", "*orkers"}, "web workers"}, {[]string{"*"}, "web workers"}, {[]string{"w{eb,orkers}"}, "web workers"},
	}
	for _, tc := range cases {
		mustSet(t, c.SetSpecificRoles(tc.patterns))
		if got := names(c.Roles()); got != tc.want {
			t.Errorf("%v: roles = %s, want %s", tc.patterns, got, tc.want)
		}
	}
}

func TestFilteringRolesByFilteringHosts(t *testing.T) {
	c := configureWith(t, "deploy_with_roles")
	mustSet(t, c.SetSpecificHosts([]string{"1.1.1.3"}))
	if names(c.Roles()) != "workers" {
		t.Fatalf("roles = %s", names(c.Roles()))
	}
}

func TestOverwritingHostsWithPrimary(t *testing.T) {
	c := configureWith(t, "deploy_with_roles")
	c.SpecificPrimary()
	if joined(c.Hosts()) != "1.1.1.1" {
		t.Fatalf("hosts = %v", c.Hosts())
	}
}

func TestPrimaryHostWithSpecificHostsViaRole(t *testing.T) {
	c := configureWith(t, "deploy_with_roles")
	mustSet(t, c.SetSpecificRoles([]string{"workers"}))
	if c.PrimaryHost() != "1.1.1.3" {
		t.Fatalf("primary host = %s", c.PrimaryHost())
	}
}

func TestPrimaryRole(t *testing.T) {
	c := configureWith(t, "deploy_with_roles")
	if c.PrimaryRole().Name() != "web" {
		t.Fatal("primary role is not web")
	}
	mustSet(t, c.SetSpecificRoles([]string{"workers"}))
	if c.PrimaryRole().Name() != "workers" {
		t.Fatal("primary role is not workers")
	}
}

func TestRolesOn(t *testing.T) {
	c := configureWith(t, "deploy_with_roles")
	if names(c.RolesOn("1.1.1.1")) != "web" || names(c.RolesOn("1.1.1.3")) != "workers" {
		t.Fatal("wrong roles on hosts")
	}
	if got := names(configureWith(t, "deploy_with_two_roles_one_host").RolesOn("1.1.1.1")); got != "web workers" {
		t.Fatalf("roles on 1.1.1.1 = %s", got)
	}
}

func TestTryToMatchThePrimaryRoleFromAListOfSpecificRoles(t *testing.T) {
	c := configureWith(t, "deploy_primary_web_role_override")
	mustSet(t, c.SetSpecificRoles([]string{"web_*"}))
	if names(c.Roles()) != "web_tokyo web_chicago" || c.PrimaryRole().Name() != "web_tokyo" || c.PrimaryHost() != "1.1.1.3" {
		t.Fatalf("roles %s primary %s host %s", names(c.Roles()), c.PrimaryRole().Name(), c.PrimaryHost())
	}
	if joined(c.Hosts()) != "1.1.1.3 1.1.1.4 1.1.1.1 1.1.1.2" {
		t.Fatalf("hosts = %v", c.Hosts())
	}
}

func TestProxyHostsObserveFilters(t *testing.T) {
	c := configureWith(t, "deploy_with_multiple_proxy_roles")
	mustSet(t, c.SetSpecificRoles([]string{"web_tokyo"}))
	if joined(c.ProxyHosts()) != "1.1.1.3 1.1.1.4" {
		t.Fatalf("proxy hosts with roles = %v", c.ProxyHosts())
	}
	c = configureWith(t, "deploy_with_multiple_proxy_roles")
	mustSet(t, c.SetSpecificHosts([]string{"1.1.1.2"}))
	if joined(c.ProxyHosts()) != "1.1.1.2" {
		t.Fatalf("proxy hosts with hosts = %v", c.ProxyHosts())
	}
}

func TestAccessoryHostsWithoutFiltering(t *testing.T) {
	if got := joined(configureWith(t, "deploy_with_single_accessory").AccessoryHosts()); got != "1.1.1.5" {
		t.Fatalf("got %s", got)
	}
	if got := joined(configureWith(t, "deploy_with_accessories_on_independent_server").AccessoryHosts()); got != "1.1.1.5 1.1.1.1 1.1.1.2" {
		t.Fatalf("got %s", got)
	}
}

func TestAccessoryHostsWithRoleFiltering(t *testing.T) {
	c := configureWith(t, "deploy_with_single_accessory")
	mustSet(t, c.SetSpecificRoles([]string{"web"}))
	if len(c.AccessoryHosts()) != 0 {
		t.Fatalf("got %v", c.AccessoryHosts())
	}
	c = configureWith(t, "deploy_with_accessories_on_independent_server")
	mustSet(t, c.SetSpecificRoles([]string{"web"}))
	if joined(c.AccessoryHosts()) != "1.1.1.1 1.1.1.2" {
		t.Fatalf("got %v", c.AccessoryHosts())
	}
	mustSet(t, c.SetSpecificRoles([]string{"workers"}))
	if len(c.AccessoryHosts()) != 0 {
		t.Fatalf("got %v", c.AccessoryHosts())
	}
}

func TestPrimaryRoleHostsAreFirst(t *testing.T) {
	c := configureWith(t, "deploy_with_roles_workers_primary")
	if joined(c.Hosts()) != "1.1.1.1 1.1.1.2 1.1.1.3 1.1.1.4" || joined(c.AppHosts()) != "1.1.1.1 1.1.1.2 1.1.1.3 1.1.1.4" {
		t.Fatalf("hosts %v app hosts %v", c.Hosts(), c.AppHosts())
	}
}

func TestWithSpecificHostsRestoresPrimaryHost(t *testing.T) {
	c := configureWith(t, "deploy_with_roles")
	var visited []string
	for _, host := range c.Hosts() {
		_ = c.WithSpecificHosts([]string{host}, func() error {
			visited = append(visited, c.PrimaryHost())
			return nil
		})
	}
	if joined(visited) != "1.1.1.1 1.1.1.2 1.1.1.3 1.1.1.4" || c.PrimaryHost() != "1.1.1.1" {
		t.Fatalf("visited %v, primary after %s", visited, c.PrimaryHost())
	}
}

func TestWithSpecificHostsRestoresPrimaryHostEvenAfterAnError(t *testing.T) {
	c := configureWith(t, "deploy_with_roles")
	boom := errors.New("test error")
	if err := c.WithSpecificHosts([]string{"1.1.1.3"}, func() error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("error = %v", err)
	}
	if c.PrimaryHost() != "1.1.1.1" {
		t.Fatalf("primary host = %s", c.PrimaryHost())
	}
}
