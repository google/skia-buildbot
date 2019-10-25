package source

import "time"

// Artifact is a single item we found during a search.
type Artifact struct {
	Title        string    `json:"title"`
	URL          string    `json:"url"`
	Hashtags     []string  `json:"hashtags"`
	LastModified time.Time `json:"last_modified"`
}

// Source is the interface each source of data must implement, i.e. Gerrit,
// Monorail, Drive, etc.
type Source interface {
	// ByHashtag returns a channel that produces Artifacts that match the given
	// hashtag.
	ByHashtag(hashtag string) <-chan Artifact

	// ByUSer returns a channel that produces Artifacts that match the given
	// email.
	ByUser(email string) <-chan Artifact
}
