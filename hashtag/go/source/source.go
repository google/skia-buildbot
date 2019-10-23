package source

import "time"

// Kind is the kind of Artifact we found.
type Kind string

const (
	// Gerrit is the kind for code reviews done in Gerrit.
	Gerrit Kind = "gerrit"

	// Monorail is the kind for bugs stored in Monorail.
	Monorail Kind = "monorail"

	// Drive is the kind for documents stored in a shared Google Drive.
	Drive Kind = "drive"

	// CodeSearch is the kind for searches over code.
	CodeSearch Kind = "codesearch"
)

// Artifact is a single item we found during a search.
type Artifact struct {
	Title        string    `json:"title"`
	URL          string    `json:"url"`
	Hashtags     []string  `json:"hashtags"`
	LastModified time.Time `json:"last_modified"`
	Kind         Kind      `json:"kind"`
}

// Source is the interface each source of data must implement, i.e. Gerrit,
// Monorail, Drive, etc.
type Source interface {
	ByHashtag(string) <-chan Artifact
	ByUser(string) <-chan Artifact
}
