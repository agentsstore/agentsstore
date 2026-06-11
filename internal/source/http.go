package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/wu/agentsstore/internal/config"
)

type HTTPSource struct {
	spec config.Source
	cli  *http.Client
}

func NewHTTPSource(spec config.Source) (*HTTPSource, error) {
	if spec.Type != "http" {
		return nil, fmt.Errorf("HTTPSource requires type=http, got %q", spec.Type)
	}
	return &HTTPSource{
		spec: spec,
		cli:  &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (h *HTTPSource) Name() string { return h.spec.Name }
func (h *HTTPSource) Type() string { return "http" }

type pluginList struct {
	Plugins []struct {
		Name   string `json:"name"`
		Source string `json:"source"`
	} `json:"plugins"`
}

func (h *HTTPSource) Fetch(ctx context.Context, destDir string) error {
	// Prepare the parent of destDir (so we can create destDir + a sibling staging dir).
	if err := os.MkdirAll(filepath.Dir(destDir), 0o755); err != nil {
		return err
	}

	// Staging directory lives next to destDir. If destDir already exists, leave it
	// alone for now; we'll remove and rename at the end on success.
	staging := destDir + ".partial"
	if err := os.RemoveAll(staging); err != nil {
		return err
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return err
	}

	if err := h.fetchInto(ctx, staging); err != nil {
		// Clean up the half-written staging dir so we never leave cruft behind.
		_ = os.RemoveAll(staging)
		return err
	}

	// Atomic swap: remove old destDir (if any), then rename staging into place.
	if err := os.RemoveAll(destDir); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	if err := os.Rename(staging, destDir); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	return nil
}

// fetchInto performs all writes into the provided directory. Any error leaves
// the directory in a half-written state; the caller is responsible for removing
// it (or replacing it via atomic rename) on failure.
func (h *HTTPSource) fetchInto(ctx context.Context, destDir string) error {
	base, err := url.Parse(h.spec.URL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	// Strip the file portion of the marketplace URL to get the base directory.
	baseDir, _ := path.Split(base.Path)
	// The directory that all plugin sources must remain under. Used for
	// traversal containment checks.
	marketplaceDir := strings.TrimSuffix(h.spec.URL, path.Base(h.spec.URL))

	data, err := h.fetch(ctx, h.spec.URL)
	if err != nil {
		return fmt.Errorf("fetch marketplace.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(destDir, "marketplace.json"), data, 0o644); err != nil {
		return err
	}

	var pl pluginList
	if err := json.Unmarshal(data, &pl); err != nil {
		// Unknown shape; nothing more to mirror.
		return nil
	}

	for _, p := range pl.Plugins {
		if p.Source == "" {
			continue
		}
		rel := p.Source
		ref, err := url.Parse(rel)
		if err != nil {
			continue
		}
		resolved := *base
		if ref.IsAbs() {
			resolved = *ref
		} else {
			// Only append /plugin.json if the source doesn't already end in it
			// (or another known manifest filename).
			cleanPath := strings.TrimSuffix(ref.Path, "/")
			if !strings.HasSuffix(cleanPath, ".json") {
				cleanPath = path.Join(cleanPath, "plugin.json")
			}
			resolved.Path = path.Join(baseDir, cleanPath)
		}

		// Ensure the resolved plugin URL is under the marketplace URL's directory.
		if !strings.HasPrefix(resolved.String(), marketplaceDir) {
			return fmt.Errorf("plugin %s: source %q escapes marketplace directory", p.Name, p.Source)
		}

		body, err := h.fetch(ctx, resolved.String())
		if err != nil {
			return fmt.Errorf("fetch plugin %s: %w", p.Name, err)
		}
		relPath := strings.TrimPrefix(resolved.Path, "/")
		if relPath == "" {
			continue
		}
		full := filepath.Join(destDir, relPath)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, body, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (h *HTTPSource) fetch(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := h.cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: %d", u, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
