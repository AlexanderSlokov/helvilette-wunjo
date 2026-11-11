// Package tags builds the tags that Kamal prints in front of audit lines and passes
// to hooks as KAMAL_* variables.
//
// Ports lib/kamal/tags.rb at Kamal 2.12.0.
package tags

import (
	"strings"
	"time"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/git"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
)

// Tag is one named value, such as {"version", "abc123"}.
type Tag struct {
	Name  string
	Value string
}

// Tags is an ordered list of tags.
type Tags []Tag

// Deploy holds the facts of a deploy that the default tags name. An empty
// Destination or AbbreviatedVersion stands for Ruby's nil.
type Deploy struct {
	Destination        string
	Version            string
	Service            string
	AbbreviatedVersion string
}

// Default returns the tags of Kamal::Tags.default_tags: recorded_at, performer,
// destination when set, version, service_version, and service.
//
//	t := tags.Default(deploy, time.Now(), tags.Performer(localshell.System{}))
func Default(d Deploy, now time.Time, performer string) Tags {
	t := Tags{{"recorded_at", now.UTC().Format("2006-01-02T15:04:05Z")}, {"performer", performer}}
	if d.Destination != "" {
		t = append(t, Tag{"destination", d.Destination})
	}
	return append(t, Tag{"version", d.Version}, Tag{"service_version", serviceVersion(d)}, Tag{"service", d.Service})
}

func serviceVersion(d Deploy) string {
	if d.AbbreviatedVersion == "" {
		return d.Service
	}
	return d.Service + "@" + d.AbbreviatedVersion
}

// Performer returns the Git email, or the output of whoami when Git has none.
func Performer(sh localshell.Shell) string {
	if email, err := (git.Git{Shell: sh}).Email(); err == nil && strings.TrimSpace(email) != "" {
		return email
	}
	result, _ := sh.Run("whoami")
	return strings.TrimSuffix(result.Stdout, "\n")
}

// With returns the tags with extra tags appended, as Kamal::Tags.from_config(config,
// **extra) does. An extra tag with an existing name replaces its value in place.
func (t Tags) With(extra ...Tag) Tags {
	out := append(Tags(nil), t...)
	for _, tag := range extra {
		out = out.set(tag)
	}
	return out
}

func (t Tags) set(tag Tag) Tags {
	for i := range t {
		if t[i].Name == tag.Name {
			t[i].Value = tag.Value
			return t
		}
	}
	return append(t, tag)
}

// Env returns each tag as KAMAL_<NAME>, for example KAMAL_VERSION=abc123.
func (t Tags) Env() []Tag {
	out := make([]Tag, len(t))
	for i, tag := range t {
		out[i] = Tag{"KAMAL_" + strings.ToUpper(tag.Name), tag.Value}
	}
	return out
}

// String returns "[value] [value] ...", as Kamal::Tags#to_s does.
func (t Tags) String() string {
	parts := make([]string, len(t))
	for i, tag := range t {
		parts[i] = "[" + tag.Value + "]"
	}
	return strings.Join(parts, " ")
}

// Except returns the tags without the named ones.
func (t Tags) Except(names ...string) Tags {
	var out Tags
	for _, tag := range t {
		if !contains(names, tag.Name) {
			out = append(out, tag)
		}
	}
	return out
}

func contains(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}
