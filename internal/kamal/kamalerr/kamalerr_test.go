package kamalerr

import (
	"errors"
	"testing"
)

func TestConfigurationErrorKeepsMessage(t *testing.T) {
	err := Configuration("Missing required configuration for image")
	var target *ConfigurationError
	if !errors.As(err, &target) || err.Error() != "Missing required configuration for image" {
		t.Fatalf("err = %v", err)
	}
}
