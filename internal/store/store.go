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

func (s *Store) SourceDir(name string) string {
	return filepath.Join(s.DataDir, "sources", name)
}

func (s *Store) AggregatedDir() string {
	return filepath.Join(s.DataDir, "aggregated")
}

func (s *Store) EnsureSourceDir(name string) error {
	return os.MkdirAll(s.SourceDir(name), 0o755)
}

func (s *Store) WriteFile(name, rel string, data []byte) error {
	full := filepath.Join(s.SourceDir(name), rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, data, 0o644)
}

func (s *Store) ReadFile(name, rel string) ([]byte, error) {
	return os.ReadFile(filepath.Join(s.SourceDir(name), rel))
}

func (s *Store) ResolvePluginPath(name, rel string) (string, error) {
	root := s.SourceDir(name)
	full := filepath.Join(root, rel)
	clean := filepath.Clean(full)
	if !strings.HasPrefix(clean, root+string(os.PathSeparator)) && clean != root {
		return "", fmt.Errorf("path %q escapes source dir", rel)
	}
	return clean, nil
}

func (s *Store) RemoveSource(name string) error {
	return os.RemoveAll(s.SourceDir(name))
}
