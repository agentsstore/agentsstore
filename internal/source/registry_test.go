package source

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wu/agentsstore/internal/config"
	"github.com/wu/agentsstore/internal/store"
)

type fakeSrc struct {
	name    string
	written bool
}

func (f *fakeSrc) Name() string { return f.name }
func (f *fakeSrc) Type() string { return "fake" }

func (f *fakeSrc) Fetch(ctx context.Context, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(destDir+"/marketplace.json", []byte(`{"plugins":[]}`), 0o644)
}

func TestRegistry_Build(t *testing.T) {
	r := NewRegistry()
	r.Register("fake", func(spec config.Source) Source { return &fakeSrc{name: spec.Name} })

	spec := config.Source{Name: "x", Type: "fake", URL: "u", Enabled: true}
	s, err := r.Build(spec)
	require.NoError(t, err)
	assert.Equal(t, "x", s.Name())

	err = s.Fetch(context.Background(), t.TempDir())
	require.NoError(t, err)
}

func TestRegistry_BuildUnknown(t *testing.T) {
	r := NewRegistry()
	_, err := r.Build(config.Source{Name: "x", Type: "nope", URL: "u", Enabled: true})
	assert.Error(t, err)
}

func TestRegistry_BuildAllFromConfig(t *testing.T) {
	r := NewRegistry()
	r.Register("fake", func(spec config.Source) Source { return &fakeSrc{name: spec.Name} })

	cfg := &config.Config{
		Server:  config.ServerConfig{DataDir: t.TempDir()},
		Sources: []config.Source{{Name: "a", Type: "fake", URL: "u", Enabled: true}, {Name: "b", Type: "fake", URL: "u", Enabled: false}},
	}
	_ = store.New(cfg.Server.DataDir)

	all, err := r.BuildAll(cfg)
	require.NoError(t, err)
	names := []string{}
	for _, s := range all {
		names = append(names, s.Name())
	}
	assert.ElementsMatch(t, []string{"a", "b"}, names)
}
