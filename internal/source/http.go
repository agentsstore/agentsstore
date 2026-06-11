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
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	base, err := url.Parse(h.spec.URL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	// Strip the file portion of the marketplace URL to get the base directory.
	baseDir, _ := path.Split(base.Path)

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
			resolved.Path = path.Join(baseDir, ref.Path, "plugin.json")
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
