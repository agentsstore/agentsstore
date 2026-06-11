package source

import (
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
)

// refFromString converts a user-provided ref string into a fully-qualified
// plumbing.ReferenceName. Accepts:
//   - "main"               -> refs/heads/main
//   - "refs/heads/main"    -> refs/heads/main
//   - "v1.0.0"             -> refs/tags/v1.0.0
//   - "refs/tags/v1.0.0"   -> refs/tags/v1.0.0
//
// Commit SHAs (40-char hex) are NOT supported by this function — they require
// a different flow (PlainClone with HEAD + ResolveRevision + Checkout).
func refFromString(s string) plumbing.ReferenceName {
	if len(s) > 0 && s[0] == '/' {
		s = s[1:]
	}
	switch {
	case strings.HasPrefix(s, "refs/heads/"):
		return plumbing.ReferenceName(s)
	case strings.HasPrefix(s, "refs/tags/"):
		return plumbing.ReferenceName(s)
	case strings.HasPrefix(s, "refs/"):
		// Other refs (refs/remotes/..., refs/notes/...) — pass through
		return plumbing.ReferenceName(s)
	default:
		// Heuristic: if it looks like a tag (contains a dot like "v1.2.3" or
		// "1.2.3"), treat as a tag. Otherwise as a branch.
		if looksLikeTag(s) {
			return plumbing.NewTagReferenceName(s)
		}
		return plumbing.NewBranchReferenceName(s)
	}
}

func looksLikeTag(s string) bool {
	// Common tag patterns: "v1.0.0", "1.0.0", "release-2024.01"
	// Be conservative — if there's a dot, treat as tag. Otherwise branch.
	return strings.Contains(s, ".")
}
