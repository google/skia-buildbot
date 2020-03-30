package deps_parser

import (
	"bytes"
	"context"

	"go.skia.org/infra/go/gitiles"
)

// FromGitiles downloads and returns the DEPS entries from the given repo at the
// given revision.
// TODO(borenet): Does this belong in this package or outside it?
func FromGitiles(ctx context.Context, repo *gitiles.Repo, rev string) (map[string]*DepsEntry, error) {
	// Load the DEPS file from the parent repo.
	var buf bytes.Buffer
	if err := repo.ReadFileAtRef(ctx, DepsFileName, rev, &buf); err != nil {
		return nil, err
	}
	return ParseDeps(buf.String())
}
