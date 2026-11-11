// Package secrets reads the .kamal/secrets files of a deploy, as Kamal::Secrets does.
// Each file is a dotenv file whose $(command) substitutions run once, on first use.
//
// Ports lib/kamal/secrets.rb and lib/kamal/secrets/dotenv/inline_command_substitution.rb
// at Kamal 2.12.0, with the parser of dotenv 3.2.0.
package secrets

import (
	"fmt"
	"strings"
	"sync"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/kamalerr"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// Files reads secrets files. It returns found false for a missing file.
type Files interface {
	Read(path string) (content []byte, found bool, err error)
}

// Secrets looks up secret values by key. It loads the files on the first lookup and
// only once, because loading can run commands that ask for input.
//
//	s := secrets.New(secrets.Options{SecretsPath: ".kamal/secrets", Files: secrets.OSFiles{}, Substitution: sub})
//	password, err := s.Fetch("KAMAL_REGISTRY_PASSWORD")
type Secrets struct {
	opts   Options
	once   sync.Once
	values *yamlord.Ordered
	found  []string
	err    error
}

// Options configures where Secrets reads from.
type Options struct {
	Destination  string
	SecretsPath  string
	Files        Files
	Substitution Substitution
}

// New returns Secrets for the given options. An empty SecretsPath means
// ".kamal/secrets".
func New(opts Options) *Secrets {
	if opts.SecretsPath == "" {
		opts.SecretsPath = ".kamal/secrets"
	}
	return &Secrets{opts: opts}
}

// Filenames returns the files Kamal reads, in order: the common file, then the file
// for the destination or the base file.
func (s *Secrets) Filenames() []string {
	main := s.opts.SecretsPath
	if s.opts.Destination != "" {
		main += "." + s.opts.Destination
	}
	return []string{s.opts.SecretsPath + "-common", main}
}

// Fetch returns the value of key, as Kamal::Secrets#[] does.
func (s *Secrets) Fetch(key string) (string, error) {
	values, err := s.load()
	if err != nil {
		return "", err
	}
	if value, ok := values.Get(key); ok {
		return value.ToS(), nil
	}
	return "", s.notFound(key)
}

func (s *Secrets) notFound(key string) error {
	if len(s.found) > 0 {
		return kamalerr.Configuration(fmt.Sprintf("Secret '%s' not found in %s", key, strings.Join(s.found, ", ")))
	}
	return kamalerr.Configuration(fmt.Sprintf("Secret '%s' not found, no secret files (%s) provided", key, strings.Join(s.Filenames(), ", ")))
}

// Has reports whether key holds a present value, as Kamal::Secrets#key? does.
func (s *Secrets) Has(key string) bool {
	values, err := s.load()
	return err == nil && values.At(key).Present()
}

// All returns every secret in file order, as Kamal::Secrets#to_h does.
func (s *Secrets) All() (*yamlord.Ordered, error) {
	values, err := s.load()
	if err != nil {
		return nil, err
	}
	return values.Clone(), nil
}

// FilesFound returns the secrets files that exist, as Kamal::Secrets#secrets_files does.
func (s *Secrets) FilesFound() ([]string, error) {
	_, err := s.load()
	return append([]string(nil), s.found...), err
}

func (s *Secrets) load() (*yamlord.Ordered, error) {
	s.once.Do(func() { s.values, s.err = s.readAll() })
	return s.values, s.err
}

// readAll merges the files in order, so the later file wins a duplicate key.
func (s *Secrets) readAll() (*yamlord.Ordered, error) {
	merged := yamlord.NewOrdered()
	for _, path := range s.Filenames() {
		values, err := s.readFile(path)
		if err != nil {
			return nil, err
		}
		merged = merged.Merge(values)
	}
	return merged, nil
}

func (s *Secrets) readFile(path string) (*yamlord.Ordered, error) {
	content, found, err := s.opts.Files.Read(path)
	if err != nil || !found {
		return yamlord.NewOrdered(), err
	}
	s.found = append(s.found, path)
	values, err := ParseDotenv(strings.TrimPrefix(string(content), "\xef\xbb\xbf"), s.opts.Substitution)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return values, nil
}
