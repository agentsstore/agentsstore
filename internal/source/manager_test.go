package source

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wu/agentsstore/internal/config"
	"github.com/wu/agentsstore/internal/store"
)

func TestManager_AddAndList(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "c.yaml")
	st := store.New(filepath.Join(dir, "data"))
	mgr := NewManager(cfgPath, st, &sync.Mutex{})

	spec := config.Source{Name: "team", Type: "git", URL: "https://x/y.git", Enabled: true}
	require.NoError(t, mgr.Add(spec))

	st2, err := mgr.List()
	require.NoError(t, err)
	assert.Len(t, st2, 1)
	assert.Equal(t, "team", st2[0].Name)

	_, err = os.Stat(cfgPath)
	assert.NoError(t, err)
}

func TestManager_Duplicate(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "c.yaml")
	st := store.New(filepath.Join(dir, "data"))
	mgr := NewManager(cfgPath, st, &sync.Mutex{})

	spec := config.Source{Name: "x", Type: "git", URL: "https://x", Enabled: true}
	require.NoError(t, mgr.Add(spec))
	err := mgr.Add(spec)
	assert.Error(t, err)
}

func TestManager_Delete(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "c.yaml")
	st := store.New(filepath.Join(dir, "data"))
	mgr := NewManager(cfgPath, st, &sync.Mutex{})

	spec := config.Source{Name: "x", Type: "git", URL: "https://x", Enabled: true}
	require.NoError(t, mgr.Add(spec))
	require.NoError(t, mgr.Delete("x"))

	st2, _ := mgr.List()
	assert.Empty(t, st2)
}

var _ = context.Background
