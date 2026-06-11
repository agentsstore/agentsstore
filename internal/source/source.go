package source

import "context"

// MarketplaceManifestPath is the well-known path, relative to the source root,
// where the Claude Code marketplace manifest lives. Both Git and HTTP sources
// must surface this file so the Aggregator can read it.
const MarketplaceManifestPath = ".claude-plugin/marketplace.json"

type Source interface {
	Name() string
	Type() string
	// Fetch downloads the source into destDir. After success,
	// destDir/.claude-plugin/marketplace.json MUST exist.
	Fetch(ctx context.Context, destDir string) error
}
