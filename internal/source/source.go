package source

import "context"

type Source interface {
	Name() string
	Type() string
	// Fetch downloads the source into destDir. After success,
	// destDir/marketplace.json MUST exist.
	Fetch(ctx context.Context, destDir string) error
}
