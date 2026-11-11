package configuration

// Ports Configuration#version, #abbreviated_version, and #git_version from
// lib/kamal/configuration.rb at Kamal 2.12.0.

import (
	"fmt"
	"strings"
)

// VersionError reports that no version is set and none can come from Git. Kamal
// raises it from deep inside the commands that need a version, so Version panics with
// it and the command-line layer recovers it.
type VersionError struct {
	WorkDir string
}

func (e *VersionError) Error() string {
	return fmt.Sprintf("Can't use commit hash as version, no git repository found in %s", e.WorkDir)
}

type versionResult struct {
	value string
	err   error
}

// SetVersion sets the declared version, as `--version` does.
func (c *Configuration) SetVersion(version string) {
	c.declaredVersion = version
	c.version = nil
}

// ResolveVersion returns the declared version, then $VERSION, then the Git revision,
// with _uncommitted_<random> when the tree has changes and Kamal builds from the
// working directory.
func (c *Configuration) ResolveVersion() (string, error) {
	if c.version == nil {
		value, err := c.computeVersion()
		c.version = &versionResult{value: value, err: err}
	}
	return c.version.value, c.version.err
}

// Version returns ResolveVersion's value and panics with a *VersionError when there
// is none, as Kamal raises.
func (c *Configuration) Version() string {
	version, err := c.ResolveVersion()
	if err != nil {
		panic(err)
	}
	return version
}

func (c *Configuration) computeVersion() (string, error) {
	if strings.TrimSpace(c.declaredVersion) != "" {
		return c.declaredVersion, nil
	}
	if version, ok := c.env.Lookup("VERSION"); ok {
		return version, nil
	}
	return c.gitVersion()
}

func (c *Configuration) gitVersion() (string, error) {
	if !c.gitUsed() {
		return "", &VersionError{WorkDir: c.env.WorkDir}
	}
	revision, err := c.env.Git.Revision()
	if err != nil {
		return "", err
	}
	changes, err := c.env.Git.UncommittedChanges()
	if err != nil {
		return "", err
	}
	if changes != "" && !c.builder.GitClone() {
		revision += "_uncommitted_" + c.env.RandomHex()
	}
	return revision, nil
}

// AbbreviatedVersion returns the first 7 characters of the version, or the whole
// version when it holds an underscore, such as <sha>_uncommitted_<random>.
func (c *Configuration) AbbreviatedVersion() string {
	version := c.Version()
	if strings.Contains(version, "_") || len(version) <= 7 {
		return version
	}
	return version[:7]
}
