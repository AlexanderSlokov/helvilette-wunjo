// Package kamalerr holds the error types that Kamal raises across modules.
//
// Ports Kamal::ConfigurationError from lib/kamal.rb at Kamal 2.12.0.
package kamalerr

// ConfigurationError reports a deploy file, secret, or option that Kamal rejects.
// Its message is the text Kamal prints, so the CLI shows it unchanged.
type ConfigurationError struct {
	Message string
}

func (e *ConfigurationError) Error() string { return e.Message }

// Configuration returns a ConfigurationError, for example
// kamalerr.Configuration("Missing required configuration for image").
func Configuration(message string) error {
	return &ConfigurationError{Message: message}
}
