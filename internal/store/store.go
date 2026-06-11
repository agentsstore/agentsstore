package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Store struct {
	DataDir string
}

func New(dataDir string) *Store {
	return &Store{DataDir: dataDir}
}

// validName rejects empty, "..", and any name containing a path separator.
// It also rejects absolute paths and ".".
func validName(name string) error {
	if name == "" {
		return fmt.Errorf("name must not be empty")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("name %q is not allowed", name)
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("name %q must not contain a path separator", name)
	}
	if filepath.IsAbs(name) {
		return fmt.Errorf("name %q must not be absolute", name)
	}
	return nil
}

// safeJoin joins root and rel, cleans the result, and verifies that the
// cleaned path is still inside root. It returns an error if rel is empty,
// absolute, or would escape root after cleaning.
func safeJoin(root, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("rel must not be empty")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("rel %q must not be absolute", rel)
	}
	full := filepath.Clean(filepath.Join(root, rel))
	if !strings.HasPrefix(full, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q escapes %q", rel, root)
	}
	return full, nil
}

func (s *Store) SourceDir(name string) string {
	return filepath.Join(s.DataDir, "sources", name)
}

func (s *Store) AggregatedDir() string {
	return filepath.Join(s.DataDir, "aggregated")
}

func (s *Store) EnsureSourceDir(name string) error {
	if err := validName(name); err != nil {
		return err
	}
	return os.MkdirAll(s.SourceDir(name), 0o755)
}

func (s *Store) WriteFile(name, rel string, data []byte) error {
	if err := validName(name); err != nil {
		return err
	}
	full, err := safeJoin(s.SourceDir(name), rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, data, 0o644)
}

func (s *Store) ReadFile(name, rel string) ([]byte, error) {
	if err := validName(name); err != nil {
		return nil, err
	}
	full, err := safeJoin(s.SourceDir(name), rel)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(full)
}

func (s *Store) ResolvePluginPath(name, rel string) (string, error) {
	if err := validName(name); err != nil {
		return "", err
	}
	return safeJoin(s.SourceDir(name), rel)
}

func (s *Store) RemoveSource(name string) error {
	if err := validName(name); err != nil {
		return err
	}
	return os.RemoveAll(s.SourceDir(name))
}
