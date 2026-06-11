package source

import (
	"github.com/go-git/go-git/v5/plumbing"
)

func refFromString(s string) plumbing.ReferenceName {
	if len(s) > 0 && s[0] == '/' {
		s = s[1:]
	}
	return plumbing.ReferenceName(s)
}
