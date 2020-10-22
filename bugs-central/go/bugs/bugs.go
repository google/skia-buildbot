package bugs

// Defines a generic interface used by the different issue frameworks.

import (
	"context"

	"go.skia.org/infra/bugs-central/go/db"
	"go.skia.org/infra/bugs-central/go/types"
)

type BugFramework interface {

	// Search calls the bug framework and returns standardized issues.
	Search(ctx context.Context) ([]*types.Issue, *types.IssueCountsData, error)

	// SearchClientAndPersist queries issues and puts results into the DB and into the
	// OpenIssues in-memory object.
	SearchClientAndPersist(ctx context.Context, dbClient *db.FirestoreDB, runId string) error

	// GetIssueLink returns a link to the specified issue ID.
	GetIssueLink(project, id string) string
}
