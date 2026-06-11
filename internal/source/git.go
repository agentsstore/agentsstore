package source

import (
	"context"
	"errors"
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
		return fmt.Errorf("mkdir dest: %w", err)
	}
	opts := &git.CloneOptions{
		URL:        g.spec.URL,
		RemoteName: "origin",
		Depth:      1,
	}
	if g.spec.Ref != "" {
		opts.ReferenceName = refFromString(g.spec.Ref)
	}

	if _, err := git.PlainCloneContext(ctx, destDir, false, opts); err != nil {
		// On any error, wipe destDir so a retry starts clean.
		_ = os.RemoveAll(destDir)
		// ErrRepositoryAlreadyExists means the directory has a repo already;
		// try to fetch + checkout the requested ref.
		if !errors.Is(err, git.ErrRepositoryAlreadyExists) {
			return fmt.Errorf("clone %s: %w", g.spec.URL, err)
		}
		repo, perr := git.PlainOpen(destDir)
		if perr != nil {
			return fmt.Errorf("open existing repo: %w", perr)
		}
		if g.spec.Ref != "" {
			if err := repo.FetchContext(ctx, &git.FetchOptions{
				RemoteName: "origin",
				Depth:      1,
				Tags:       git.AllTags,
			}); err != nil {
				return fmt.Errorf("fetch: %w", err)
			}
			w, werr := repo.Worktree()
			if werr != nil {
				return fmt.Errorf("worktree: %w", werr)
			}
			if err := w.Checkout(&git.CheckoutOptions{
				Branch: refFromString(g.spec.Ref),
				Force:  true,
			}); err != nil {
				return fmt.Errorf("checkout: %w", err)
			}
		}
	}
	return nil
}
