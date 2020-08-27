package firestore

import (
	"context"

	"cloud.google.com/go/firestore"
)

// DefaultSearchLimit is the default maximum number of results returned from
// Search().
const DefaultSearchLimit = 100

// WhereClause is a struct which helps with querying Firestore.
type WhereClause struct {
	Path  string      `json:"path"`
	Op    string      `json:"op"`
	Value interface{} `json:"value"`
}

// Where returns a WhereClause for the given inputs.
func Where(path, op string, value interface{}) WhereClause {
	return WhereClause{Path: path, Op: op, Value: value}
}

// Apply the where-term to the given Query.
func (w WhereClause) Apply(q firestore.Query) firestore.Query {
	return q.Where(w.Path, w.Op, w.Value)
}

// Search performs a search using the given base query and additional where-
// clauses.
func Search(ctx context.Context, q firestore.Query, limit int, cursor string, terms ...WhereClause) (string, []*firestore.DocumentSnapshot, error) {
	for _, term := range terms {
		q = term.Apply(q)
	}
	if cursor != "" {
		q = q.StartAfter(cursor)
	}
	if limit <= 0 {
		limit = DefaultSearchLimit
	}
	q = q.Limit(limit)

	docs, err := q.Documents(ctx).GetAll()
	if err != nil {
		return "", nil, err
	}

	// Note: if we got fewer results than the requested limit, we can assume
	// that there are no more results to be seen.  However, if we received the
	// same number of results as the limit, we can't know whether we've reached
	// the end of the results or just hit the limit, so we have to assume the
	// latter.  This will result in an extra call to Search() in rare (1/limit)
	// cases.
	cursor = ""
	if len(docs) == limit {
		cursor = docs[len(docs)-1].Ref.ID
	}
	return cursor, docs, nil
}
