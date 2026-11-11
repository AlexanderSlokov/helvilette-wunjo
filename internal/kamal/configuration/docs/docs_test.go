package docs

import (
	"strings"
	"testing"
)

func TestReadReturnsEachSection(t *testing.T) {
	sections := []string{"accessory", "alias", "boot", "builder", "configuration", "env", "logging", "output", "proxy", "registry", "role", "servers", "ssh", "sshkit"}
	for _, section := range sections {
		content, err := Read(section)
		if err != nil || len(content) == 0 {
			t.Fatalf("Read(%q) = %d bytes, %v", section, len(content), err)
		}
	}
}

func TestReadRejectsUnknownSection(t *testing.T) {
	_, err := Read("nope")
	if err == nil || !strings.Contains(err.Error(), `"nope"`) {
		t.Fatalf("Read(nope) error = %v, want one naming the section", err)
	}
}
