package bugs

// Defines a generic interface used by the different issue frameworks.

import (
	"context"

	"go.skia.org/infra/bugs-central/go/db"
	"go.skia.org/infra/bugs-central/go/types"
)

type BugFramework interface {

	// Search returns issues that match the provided parameters.
	Search(ctx context.Context, config interface{}) ([]*types.Issue, *types.IssueCountsData, error)

	// SearchClientAndPersist queries issues that match the provided client parameters.
	// The results should be put into the DB and into the OpenIssues in-memory object.
	SearchClientAndPersist(ctx context.Context, config interface{}, dbClient *db.FirestoreDB, runId string) error

	// GetIssueLink returns a link to the specified issue ID.
	GetIssueLink(project, id string) string
}
