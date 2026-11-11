package secrets

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// OSFiles reads secrets files from disk, relative to Dir when the path is relative.
type OSFiles struct {
	Dir string
}

// Read returns the content of path, or found false when the file does not exist.
func (f OSFiles) Read(path string) ([]byte, bool, error) {
	if f.Dir != "" && !filepath.IsAbs(path) {
		path = filepath.Join(f.Dir, path)
	}
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	return content, err == nil, err
}

// FakeFiles holds secrets files in memory for tests, keyed by path.
//
//	files := secrets.FakeFiles{".kamal/secrets": "SECRET=ABC"}
type FakeFiles map[string]string

// Read returns the stored content of path.
func (f FakeFiles) Read(path string) ([]byte, bool, error) {
	content, ok := f[path]
	return []byte(content), ok, nil
}
