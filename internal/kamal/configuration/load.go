package configuration

// Ports Configuration.create_from and its file loading from
// lib/kamal/configuration.rb at Kamal 2.12.0.

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/erb"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// CreateFrom reads configFile, deep-merges the destination file over it when a
// destination is set, and builds the configuration. Inside the ERB of both files,
// ENV["KAMAL_DESTINATION"] reads the destination, as Kamal sets it before rendering.
//
//	cfg, err := configuration.CreateFrom("config/deploy.yml", configuration.Options{Destination: "staging", Env: env})
func CreateFrom(configFile string, opts Options) (*Configuration, error) {
	opts.Env = withDefaults(opts.Env)
	opts.Env.Lookup = withDestination(opts.Env.Lookup, opts.Destination)
	raw, err := LoadRawConfig(configFile, opts.Destination, opts.Env)
	if err != nil {
		return nil, err
	}
	return New(raw, opts)
}

// withDestination answers KAMAL_DESTINATION with the destination and leaves it unset
// without one, as ENV["KAMAL_DESTINATION"] = destination does in Ruby.
func withDestination(lookup func(string) (string, bool), destination string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		if name != "KAMAL_DESTINATION" {
			return lookup(name)
		}
		return destination, destination != ""
	}
}

// LoadRawConfig returns the merged, rendered deploy files.
func LoadRawConfig(configFile, destination string, env Environment) (*yamlord.Ordered, error) {
	files := []string{configFile}
	if destination != "" {
		files = append(files, DestinationConfigFile(configFile, destination))
	}
	merged := yamlord.NewOrdered()
	for _, file := range files {
		loaded, err := loadConfigFile(file, env)
		if err != nil {
			return nil, err
		}
		merged = merged.DeepMerge(loaded)
	}
	return merged, nil
}

// DestinationConfigFile returns config/deploy.<destination>.yml for
// config/deploy.yml, as Pathname#sub_ext does.
func DestinationConfigFile(configFile, destination string) string {
	return strings.TrimSuffix(configFile, filepath.Ext(configFile)) + "." + destination + ".yml"
}

func loadConfigFile(file string, env Environment) (*yamlord.Ordered, error) {
	if !env.Files.Exists(file) {
		return nil, fmt.Errorf("Configuration file not found in %s", file)
	}
	template, err := env.Files.ReadFile(file)
	if err != nil {
		return nil, err
	}
	rendered, err := erb.Render(string(template), erb.Options{File: file, TrimMode: true, Lookup: env.Lookup})
	if err != nil {
		return nil, err
	}
	return parseConfigMapping(file, rendered)
}

func parseConfigMapping(file, rendered string) (*yamlord.Ordered, error) {
	doc, err := yamlord.Parse([]byte(rendered))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if doc.Map() == nil {
		return nil, fmt.Errorf("%s: want a mapping of configuration keys, got %s", file, doc.Inspect())
	}
	return doc.Map(), nil
}
