package revision

import (
	"fmt"
	"time"

	"go.skia.org/infra/go/vcsinfo"
)

// Revision is a struct containing information about a given revision.
type Revision struct {
	// Id is the full ID of this Revision, eg. a full commit hash. This is
	// the only required field.
	Id string `json:"id"`

	// Display is a string used for human-friendly display of the Revision,
	// eg. a shortened commit hash.
	Display string `json:"display"`

	// Description is a human-friendly description of the Revision, eg. a
	// commit title line.
	Description string `json:"description"`

	// Timestamp is the time at which the Revision was created.
	Timestamp time.Time `json:"time"`

	// URL used by a human to view the Revision.
	URL string `json:"url"`
}

// Copy the Revision.
func (r *Revision) Copy() *Revision {
	return &Revision{
		Id:          r.Id,
		Display:     r.Display,
		Description: r.Description,
		Timestamp:   r.Timestamp,
		URL:         r.URL,
	}
}

// Implement the Stringer interface for prettier printing.
func (r *Revision) String() string {
	if r.Display != "" {
		return r.Display
	}
	return r.Id
}

// FromLongCommit converts a vcsinfo.LongCommit to a Revision. If revLinkTmpl is
// not provided, the Revision will have no URL.
func FromLongCommit(revLinkTmpl string, c *vcsinfo.LongCommit) *Revision {
	revLink := ""
	if revLinkTmpl != "" {
		revLink = fmt.Sprintf(revLinkTmpl, c.Hash)
	}
	return &Revision{
		Id:          c.Hash,
		Display:     c.Hash[:7],
		Description: c.Subject,
		Timestamp:   c.Timestamp,
		URL:         revLink,
	}
}

// FromLongCommits converts a slice of vcsinfo.LongCommits to a slice of
// Revisions. If revLinkTmpl is not provided, the Revisions will have no URL.
func FromLongCommits(revLinkTmpl string, commits []*vcsinfo.LongCommit) []*Revision {
	rv := make([]*Revision, 0, len(commits))
	for _, c := range commits {
		rv = append(rv, FromLongCommit(revLinkTmpl, c))
	}
	return rv
}
