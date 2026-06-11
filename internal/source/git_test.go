package source

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wu/agentsstore/internal/config"
)

// initBareRepo creates a local bare repo and a worktree with one commit
// containing marketplace.json. Returns the clone URL.
func initBareRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bare := filepath.Join(dir, "bare.git")
	work := filepath.Join(dir, "work")

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = work
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	require.NoError(t, os.MkdirAll(work, 0o755))
	run("init", "-q", "-b", "main")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")
	require.NoError(t, os.WriteFile(filepath.Join(work, "marketplace.json"),
		[]byte(`{"plugins":[{"name":"p1","source":"./plugins/p1"}]}`), 0o644))
	run("add", ".")
	run("commit", "-q", "-m", "init")

	clone := exec.Command("git", "clone", "-q", "--bare", work, bare)
	out, err := clone.CombinedOutput()
	require.NoError(t, err, string(out))
	return bare
}

func TestGitSource_Fetch(t *testing.T) {
	url := initBareRepo(t)
	dest := t.TempDir()

	spec := config.Source{Name: "g", Type: "git", URL: url, Ref: "main", Enabled: true}
	src, err := NewGitSource(spec)
	require.NoError(t, err)

	require.NoError(t, src.Fetch(context.Background(), dest))

	data, err := os.ReadFile(filepath.Join(dest, "marketplace.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "p1")
}
