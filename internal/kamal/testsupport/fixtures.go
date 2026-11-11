// Package testsupport holds helpers that the ported Kamal tests share: fixture paths,
// the ArchHelper replacement, and the fixed architectures the tests assume.
//
// Ports the ArchHelper module and copy_fixtures from test/test_helper.rb at
// Kamal 2.12.0.
package testsupport

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// LocalArch is the architecture that a test's fake Docker reports for the local
// machine. Kamal's ArchHelper asks the real Docker. The port fixes the value so that
// tests are repeatable on every machine.
const LocalArch = "amd64"

// RemoteArch is the architecture that Kamal's ArchHelper.remote_arch returns when the
// local architecture is LocalArch: the other of amd64 and arm64.
const RemoteArch = "arm64"

// RemoteArchVariable replaces ArchHelper.remote_arch in fixtures, because Wunjo's ERB
// subset cannot call Ruby methods.
const RemoteArchVariable = "WUNJO_TEST_REMOTE_ARCH"

// FixturesDir returns the absolute path of internal/kamal/testdata/fixtures.
//
//	path := filepath.Join(testsupport.FixturesDir(), "deploy.yml")
func FixturesDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "testdata", "fixtures")
}

// ReadFixture returns a fixture with each ArchHelper.remote_arch call replaced by
// ENV["WUNJO_TEST_REMOTE_ARCH"]. Pair it with a lookup that sets that variable to
// RemoteArch.
//
//	template, err := testsupport.ReadFixture("deploy_with_remote_builder.yml")
func ReadFixture(name string) (string, error) {
	content, err := os.ReadFile(filepath.Join(FixturesDir(), name))
	if err != nil {
		return "", err
	}
	return ReplaceArchHelper(string(content)), nil
}

// ReplaceArchHelper rewrites the ArchHelper.remote_arch call that six Kamal fixtures
// use into an ERB form the subset supports.
func ReplaceArchHelper(template string) string {
	return strings.ReplaceAll(template, "ArchHelper.remote_arch", `ENV["`+RemoteArchVariable+`"]`)
}
