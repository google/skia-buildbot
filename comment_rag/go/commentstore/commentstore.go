package commentstore

import (
	"context"

	"cloud.google.com/go/spanner"
	"go.opencensus.io/trace"
	spannerSchema "go.skia.org/infra/comment_rag/go/spanner"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
)

// CommentRecord represents a single row in the Spanner table.
type CommentRecord struct {
	ID            string
	Project       string
	Repo          string
	Category      string
	ChangeID      int64
	FilePath      string
	CommentText   string
	CodeSnippet   string
	CLSubject     string
	CLDescription string
	Analysis      string
	Embedding     []float32
}

// FoundCommentRecord represents a record retrieved during vector similarity search.
type FoundCommentRecord struct {
	CommentRecord
	Distance float64
}

// CommentStore defines an interface for interacting with Spanner for comment RAG data.
type CommentStore interface {
	// SearchComments searches for the most relevant comment records for the given query embedding.
	SearchComments(ctx context.Context, queryEmbedding []float32, maxComments int, project, repo string, categories []string) ([]*FoundCommentRecord, error)

	// WriteCommentRecord writes a comment record to Spanner. Useful for seeding and testing.
	WriteCommentRecord(ctx context.Context, c *CommentRecord) error
}

type spannerCommentStoreImpl struct {
	spannerClient *spanner.Client
	searchMetrics metrics2.Timer
	writeMetrics  metrics2.Timer
}

// NewSpannerCommentStore returns a new CommentStore instance that uses Cloud Spanner.
func NewSpannerCommentStore(spannerClient *spanner.Client) CommentStore {
	return &spannerCommentStoreImpl{
		spannerClient: spannerClient,
		searchMetrics: metrics2.NewTimer("comment_rag_search_comments"),
		writeMetrics:  metrics2.NewTimer("comment_rag_write_comments"),
	}
}

// SearchComments implements vector search using COSINE_DISTANCE inside Cloud Spanner.
func (s *spannerCommentStoreImpl) SearchComments(ctx context.Context, queryEmbedding []float32, maxComments int, project, repo string, categories []string) ([]*FoundCommentRecord, error) {
	s.searchMetrics.Start()
	defer s.searchMetrics.Stop()

	ctx, span := trace.StartSpan(ctx, "commentrag.commentstore.SearchComments")
	defer span.End()

	query := `
		SELECT
			ch.id,
			ch.project,
			ch.repo,
			ch.category,
			ch.change_id,
			ch.file_path,
			ch.comment_text,
			ch.code_snippet,
			cl.cl_subject,
			cl.cl_description,
			ch.analysis,
			COSINE_DISTANCE(ch.embedding, @queryEmbedding) as distance
		FROM
			CommentHistory ch
		LEFT JOIN
			CLInfo cl ON ch.project = cl.project AND ch.repo = cl.repo AND ch.change_id = cl.change_id
	`

	var filters []string
	if project != "" {
		filters = append(filters, "ch.project = @project")
	}
	if repo != "" {
		filters = append(filters, "ch.repo = @repo")
	}
	if len(categories) > 0 {
		filters = append(filters, "ch.category IN UNNEST(@categories)")
	}

	if len(filters) > 0 {
		query += " WHERE " + filters[0]
		for i := 1; i < len(filters); i++ {
			query += " AND " + filters[i]
		}
	}

	query += `
		ORDER BY
			distance
		LIMIT @limit
	`

	stmt := spanner.NewStatement(query)
	stmt.Params["queryEmbedding"] = queryEmbedding
	stmt.Params["limit"] = int64(maxComments)
	if project != "" {
		stmt.Params["project"] = project
	}
	if repo != "" {
		stmt.Params["repo"] = repo
	}
	if len(categories) > 0 {
		stmt.Params["categories"] = categories
	}

	var ret []*FoundCommentRecord
	err := s.spannerClient.Single().Query(ctx, stmt).Do(func(r *spanner.Row) error {
		var c CommentRecord
		var distance float64

		if err := r.ColumnByName("id", &c.ID); err != nil {
			return skerr.Wrap(err)
		}
		if err := r.ColumnByName("project", &c.Project); err != nil {
			return skerr.Wrap(err)
		}
		if err := r.ColumnByName("repo", &c.Repo); err != nil {
			return skerr.Wrap(err)
		}
		if err := r.ColumnByName("category", &c.Category); err != nil {
			return skerr.Wrap(err)
		}
		if err := r.ColumnByName("change_id", &c.ChangeID); err != nil {
			return skerr.Wrap(err)
		}
		if err := r.ColumnByName("file_path", &c.FilePath); err != nil {
			return skerr.Wrap(err)
		}
		if err := r.ColumnByName("comment_text", &c.CommentText); err != nil {
			return skerr.Wrap(err)
		}
		if err := r.ColumnByName("code_snippet", &c.CodeSnippet); err != nil {
			return skerr.Wrap(err)
		}
		if err := r.ColumnByName("cl_subject", &c.CLSubject); err != nil {
			return skerr.Wrap(err)
		}
		if err := r.ColumnByName("cl_description", &c.CLDescription); err != nil {
			return skerr.Wrap(err)
		}
		if err := r.ColumnByName("analysis", &c.Analysis); err != nil {
			return skerr.Wrap(err)
		}
		if err := r.ColumnByName("distance", &distance); err != nil {
			return skerr.Wrap(err)
		}

		ret = append(ret, &FoundCommentRecord{
			CommentRecord: c,
			Distance:      distance,
		})
		return nil
	})

	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return ret, nil
}

// WriteCommentRecord writes a comment record to Spanner. Useful for seeding and testing.
func (s *spannerCommentStoreImpl) WriteCommentRecord(ctx context.Context, c *CommentRecord) error {
	s.writeMetrics.Start()
	defer s.writeMetrics.Stop()

	ctx, span := trace.StartSpan(ctx, "commentrag.commentstore.WriteCommentRecord")
	defer span.End()

	if c.Category != "" && !spannerSchema.IsValidCategory(c.Category) {
		return skerr.Fmt("cannot write CommentRecord with invalid category: %q. Supported categories are: %v", c.Category, spannerSchema.ValidCategories)
	}

	_, err := s.spannerClient.ReadWriteTransaction(ctx, func(ctx context.Context, rwt *spanner.ReadWriteTransaction) error {
		mCL := spanner.InsertOrUpdateMap("CLInfo", map[string]interface{}{
			"project":        c.Project,
			"repo":           c.Repo,
			"change_id":      c.ChangeID,
			"cl_subject":     c.CLSubject,
			"cl_description": c.CLDescription,
		})
		mCH := spanner.InsertOrUpdateMap("CommentHistory", map[string]interface{}{
			"id":           c.ID,
			"project":      c.Project,
			"repo":         c.Repo,
			"category":     c.Category,
			"change_id":    c.ChangeID,
			"file_path":    c.FilePath,
			"comment_text": c.CommentText,
			"code_snippet": c.CodeSnippet,
			"analysis":     c.Analysis,
			"embedding":    c.Embedding,
		})
		return rwt.BufferWrite([]*spanner.Mutation{mCL, mCH})
	})
	return skerr.Wrap(err)
}
