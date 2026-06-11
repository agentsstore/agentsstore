package source

import (
	"context"
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/wu/agentsstore/internal/config"
)

type GitSource struct {
	spec config.Source
}

func NewGitSource(spec config.Source) (*GitSource, error) {
	if spec.Type != "git" {
		return nil, fmt.Errorf("GitSource requires type=git, got %q", spec.Type)
	}
	return &GitSource{spec: spec}, nil
}

func (g *GitSource) Name() string { return g.spec.Name }
func (g *GitSource) Type() string { return "git" }

func (g *GitSource) Fetch(ctx context.Context, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	opts := &git.CloneOptions{
		URL:        g.spec.URL,
		RemoteName: "origin",
	}
	if g.spec.Ref != "" {
		opts.ReferenceName = refFromString(g.spec.Ref)
	}
	opts.Depth = 1

	if _, err := git.PlainCloneContext(ctx, destDir, false, opts); err != nil {
		// If already cloned (Pull), try a fetch + checkout.
		repo, perr := git.PlainOpen(destDir)
		if perr != nil {
			return fmt.Errorf("clone failed (%v) and open failed (%v)", err, perr)
		}
		if g.spec.Ref != "" {
			_ = repo.FetchContext(ctx, &git.FetchOptions{RemoteName: "origin", Depth: 1, Tags: git.AllTags})
			w, _ := repo.Worktree()
			_ = w.Checkout(&git.CheckoutOptions{Branch: refFromString(g.spec.Ref), Force: true})
		}
	}
	return nil
}
