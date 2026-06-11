package aggregator

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/wu/agentsstore/internal/source"
	"github.com/wu/agentsstore/internal/store"
)

type Aggregator struct {
	store   *store.Store
	baseURL string
}

func New(s *store.Store, baseURL string) *Aggregator {
	return &Aggregator{store: s, baseURL: strings.TrimRight(baseURL, "/")}
}

type plugin struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

type manifest struct {
	Name    string   `json:"name,omitempty"`
	Plugins []plugin `json:"plugins"`
}

// Refresh rewrites the per-source marketplace.json files into a single
// aggregated marketplace.json. The optional baseURL parameter overrides the
// constructor's baseURL for this refresh, allowing per-request base URLs
// (e.g., derived from the request Host). If baseURL is empty, the
// constructor's baseURL is used.
func (a *Aggregator) Refresh(sourceNames []string, baseURL string) error {
	effective := a.baseURL
	if baseURL != "" {
		effective = strings.TrimRight(baseURL, "/")
	}
	merged := manifest{Plugins: []plugin{}}
	for _, name := range sourceNames {
		data, err := a.store.ReadFile(name, source.MarketplaceManifestPath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf(
					"source %s: no %s at the source root\n"+
						"  the Claude Code marketplace format requires %s\n"+
						"  to be present in the source repository. Common causes:\n"+
						"    - the repository is not a marketplace (e.g., it is a plugin, skill, or library)\n"+
						"    - the configured ref points to a branch/tag that lacks the file\n"+
						"    - the clone did not complete (check the source's status in the admin UI)",
					name, source.MarketplaceManifestPath, source.MarketplaceManifestPath)
			}
			return fmt.Errorf("source %s: read %s: %w", name, source.MarketplaceManifestPath, err)
		}
		var m manifest
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("source %s: parse %s (not valid JSON?): %w", name, source.MarketplaceManifestPath, err)
		}
		for _, p := range m.Plugins {
			p.Source = rewriteURL(effective, name, p.Source)
			merged.Plugins = append(merged.Plugins, p)
		}
	}
	if err := os.MkdirAll(a.store.AggregatedDir(), 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(a.store.AggregatedDir(), "marketplace.json"), out, 0o644)
}

func rewriteURL(baseURL, srcName, pluginSrc string) string {
	if u, err := url.Parse(pluginSrc); err == nil && u.IsAbs() {
		// External URL — leave as-is so consumer can fetch directly
		return pluginSrc
	}
	clean := strings.TrimPrefix(pluginSrc, "./")
	return fmt.Sprintf("%s/plugins/%s/%s", baseURL, srcName, path.Clean(clean))
}
