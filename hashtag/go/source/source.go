package source

import (
	"context"
	"time"
)

// Artifact is a single item we found during a search.
type Artifact struct {
	Title        string    `json:"title"`
	URL          string    `json:"url"`
	LastModified time.Time `json:"last_modified"`

	// These values below are not stored or returned from searches, they are
	// populated by the server to be used in templating pages.

	// Hidden is false if the result should be shown.
	Hidden bool `json:"hidden"`

	// Value is the search value (hashtag or email address) that produced this result.
	Value string `json:"value"`
}

// QueryType is the type of a Query.
type QueryType string

const (
	// HashtagQuery - The Value is Query is a hashtag.
	HashtagQuery QueryType = "hashtag"

	// UserQuery - The Value in Query is an email address.
	UserQuery QueryType = "email"
)

// Query to be given to a Source to do a search.
type Query struct {

	// Type indicates what is stored in Value.
	Type QueryType

	// Value can be either a hashtag or an email address depending on Type.
	Value string

	// Begin - If not zero the results last modified time should not appear
	// before this value.
	Begin time.Time

	// End - If not zero the results last modified time should not appear after
	// this value.
	End time.Time
}

// Source is the interface each source of data must implement, i.e. Gerrit,
// Monorail, Drive, etc.
type Source interface {
	// Search returns a channel that produces Artifacts that match the given
	// Query.
	Search(ctx context.Context, q Query) <-chan Artifact
}
