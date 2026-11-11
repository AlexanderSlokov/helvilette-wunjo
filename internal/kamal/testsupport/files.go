package testsupport

import "fmt"

// FakeFiles holds files in memory for tests, keyed by path. It satisfies
// secrets.Files and configuration.FileSystem, so tests never touch the disk.
//
//	files := testsupport.FakeFiles{".kamal/secrets": "SECRET=ABC"}
type FakeFiles map[string]string

// Read returns the content of path and whether it exists.
func (f FakeFiles) Read(path string) ([]byte, bool, error) {
	content, ok := f[path]
	return []byte(content), ok, nil
}

// ReadFile returns the content of path, or an error naming the missing path.
func (f FakeFiles) ReadFile(path string) ([]byte, error) {
	content, ok := f[path]
	if !ok {
		return nil, fmt.Errorf("testsupport.FakeFiles: no file %q", path)
	}
	return []byte(content), nil
}

// Exists reports whether path is stored.
func (f FakeFiles) Exists(path string) bool {
	_, ok := f[path]
	return ok
}
