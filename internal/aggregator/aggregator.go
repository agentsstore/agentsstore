package aggregator

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"

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

func (a *Aggregator) Refresh(sourceNames []string) error {
	merged := manifest{Plugins: []plugin{}}
	for _, name := range sourceNames {
		data, err := a.store.ReadFile(name, "marketplace.json")
		if err != nil {
			return fmt.Errorf("source %s: %w", name, err)
		}
		var m manifest
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("source %s: parse: %w", name, err)
		}
		for _, p := range m.Plugins {
			p.Source = rewriteURL(a.baseURL, name, p.Source)
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
	return a.store.WriteFile("aggregated", "marketplace.json", out)
}

func rewriteURL(baseURL, srcName, pluginSrc string) string {
	if u, err := url.Parse(pluginSrc); err == nil && u.IsAbs() {
		// External URL — leave as-is so consumer can fetch directly
		return pluginSrc
	}
	clean := strings.TrimPrefix(pluginSrc, "./")
	return fmt.Sprintf("%s/plugins/%s/%s", baseURL, srcName, path.Clean(clean))
}
