package source

import (
	"regexp"
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

// sshGitURLRe matches SSH-style git URLs of the form:
//   git@github.com:owner/repo.git
//   git@gitlab.com:group/subgroup/repo.git
//   user@example.com:path/to/repo.git
var sshGitURLRe = regexp.MustCompile(`^[\w-]+@([^:]+):(.+?)/?$`)

// normalizeGitURL converts SSH-style git URLs from known public hosts
// (github.com, gitlab.com) to their public HTTPS equivalents, so the
// aggregator server can clone without needing an SSH agent or credentials.
//
//	git@github.com:owner/repo.git   -> https://github.com/owner/repo.git
//	git@gitlab.com:owner/repo.git   -> https://gitlab.com/owner/repo.git
//	git@example.com:foo/bar.git     -> git@example.com:foo/bar.git  (unchanged, unknown host)
//
// Local paths (no host) and already-HTTPS URLs are returned unchanged.
func normalizeGitURL(u string) string {
	m := sshGitURLRe.FindStringSubmatch(u)
	if m == nil {
		return u
	}
	host, rest := m[1], m[2]
	switch strings.ToLower(host) {
	case "github.com", "gitlab.com", "bitbucket.org":
		return "https://" + host + "/" + rest
	default:
		// Unknown host — leave as SSH; user probably has a private host
		// configured and will need to set up an SSH key/agent themselves.
		return u
	}
}
