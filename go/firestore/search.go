package firestore

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"time"

	"cloud.google.com/go/firestore"
	"go.skia.org/infra/go/skerr"
)

func init() {
	gob.Register(time.Time{})
}

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

// Query is a wrapper for firestore.Query which helps with searching.
type Query struct {
	q       firestore.Query
	cursor  string
	limit   int
	orderBy []string
	done    bool
}

// NewQuery returns a Query instance.
func NewQuery(coll *firestore.CollectionRef) Query {
	return Query{
		q:       coll.Query,
		cursor:  "",
		limit:   DefaultSearchLimit,
		orderBy: []string{},
		done:    false,
	}
}

// Where filters the results of the Query.
func (q Query) Where(path, op string, value interface{}) Query {
	return Query{
		q:       q.q.Where(path, op, value),
		cursor:  q.cursor,
		limit:   q.limit,
		orderBy: q.orderBy,
	}
}

// WhereAll applies all of the given WhereClauses to the Query.
func (q Query) WhereAll(wheres ...WhereClause) Query {
	rv := q
	for _, w := range wheres {
		rv = rv.Where(w.Path, w.Op, w.Value)
	}
	return rv
}

// OrderBy determines the sort order of the Query.
func (q Query) OrderBy(path string, dir firestore.Direction) Query {
	return Query{
		q:       q.q.OrderBy(path, dir),
		cursor:  q.cursor,
		limit:   q.limit,
		orderBy: append(q.orderBy, path),
	}
}

// Limit limits the number of elements returned by the Query. If the given limit
// is less than 1, it is ignored.
func (q Query) Limit(n int) Query {
	if n < 1 {
		return q
	}
	return Query{
		q:       q.q.Limit(n),
		cursor:  q.cursor,
		limit:   n,
		orderBy: q.orderBy,
	}
}

// Cursor sets the given cursor on the Query.
func (q Query) Cursor(cursor string) Query {
	return Query{
		q:       q.q,
		cursor:  cursor,
		limit:   q.limit,
		orderBy: q.orderBy,
	}
}

// GetCursor retrieves the Query cursor, if any.
func (q Query) GetCursor() string {
	return q.cursor
}

// Done returns true if there are no more results in this Query.
func (q Query) Done() bool {
	return q.done
}

// packCursor encodes the given values into a cursor.
func packCursor(vals []interface{}) (string, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(vals); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// unpackCursor decodes the given cursor into individual values.
func unpackCursor(cursor string) ([]interface{}, error) {
	gobBytes, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return nil, skerr.Wrapf(err, "invalid cursor value %q", cursor)
	}
	var vals []interface{}
	if err := gob.NewDecoder(bytes.NewReader(gobBytes)).Decode(&vals); err != nil {
		return nil, skerr.Wrapf(err, "invalid cursor value %q", cursor)
	}
	return vals, nil
}

// Search performs a search and returns the results and an updated Query which
// can be used to retrieve the next page of results. Not thread safe.
func (q Query) Search(ctx context.Context) ([]*firestore.DocumentSnapshot, Query, error) {
	// If a cursor was provided, decode it and apply it to the query.
	fsQuery := q.q
	if q.cursor != "" {
		cursorVals, err := unpackCursor(q.cursor)
		if err != nil {
			return nil, Query{}, skerr.Wrap(err)
		}
		fsQuery = fsQuery.StartAfter(cursorVals...)
	}

	// Execute the query.
	docs, err := fsQuery.Documents(ctx).GetAll()
	if err != nil {
		return nil, Query{}, err
	}

	// Mark the Query as done. If there are more results, the below call to
	// Cursor() will unmark it.
	q.cursor = ""
	q.done = true

	// If necessary, encode a cursor.
	// NOTE: if we got fewer results than the requested limit, we can assume
	// that there are no more results to be seen.  However, if we received the
	// same number of results as the limit, we can't know whether we've reached
	// the end of the results or just hit the limit, so we have to assume the
	// latter.  This will result in an extra call to Search() in rare (1/limit)
	// cases.
	if len(docs) == q.limit {
		data := docs[len(docs)-1].Data()
		vals := make([]interface{}, 0, len(q.orderBy))
		for _, path := range q.orderBy {
			vals = append(vals, data[path])
		}
		cursor, err := packCursor(vals)
		if err != nil {
			return nil, Query{}, skerr.Wrap(err)
		}
		q = q.Cursor(cursor)
	}
	return docs, q, nil
}
