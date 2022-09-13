package revision

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	projectChromium     = "chromium"
	bugProjectDefault   = projectChromium
	BugProjectBuganizer = "buganizer"
	bugsPattern         = `^(?:BUG=|Bug:)\s*((?:b\/|\w+\:)?\d*(?:\s*(?:,|\s)\s*(?:b\/|\w+\:)?\d*)*)\s*$`
)

var (
	bugsRegex = regexp.MustCompile(bugsPattern)
	testsRe   = regexp.MustCompile("(?m)^Test: *(.*) *$")
)

// Revision is a struct containing information about a given revision.
type Revision struct {
	// Id is the full ID of this Revision, eg. a full commit hash. This is
	// the only required field.
	Id string `json:"id"`

	// Author is a string indicating the author of this Revision.
	Author string `json:"author"`

	// ExternalChangeId is the external change ID that, if specified, is included
	// as part of the roll. The ID is defined by the repo_manager.
	// Eg: CL num for Chromium, PR num for Github, Topic name for Android.
	ExternalChangeId string `json:"external_change_id"`

	// Bugs are the IDs of any bugs referenced by this Revision, keyed by
	// project ID (defined in whatever way makes sense to the user).
	Bugs map[string][]string `json:"bugs"`

	// Dependencies are revision IDs of dependencies of this Revision, keyed
	// by dependency ID (defined in whatever way makes sense to the user).
	Dependencies map[string]string `json:"dependencies"`

	// Description is a brief, human-friendly description of the Revision,
	// eg. a commit title line.
	Description string `json:"description"`

	// Details contains a full description of the Revision, eg. a git commit
	// body.
	Details string `json:"details"`

	// Display is a string used for human-friendly display of the Revision,
	// eg. a shortened commit hash.
	Display string `json:"display"`

	// InvalidReason indicates we should not roll to this Revision and why,
	// if it is non-empty. Note that rolls may still *include* this
	// Revision, eg. if this is a git commit and we roll to a descendant of
	// it.
	InvalidReason string `json:"invalidReason"`

	// Tests are any tests which should be run on rolls including this
	// Revision.
	Tests []string `json:"tests"`

	// Timestamp is the time at which the Revision was created.
	Timestamp time.Time `json:"time"`

	// URL used by a human to view the Revision.
	URL string `json:"url"`
}

// Copy the Revision.
func (r *Revision) Copy() *Revision {
	var bugs map[string][]string
	if r.Bugs != nil {
		bugs = make(map[string][]string, len(r.Bugs))
		for k, v := range r.Bugs {
			bugs[k] = util.CopyStringSlice(v)
		}
	}
	return &Revision{
		Id:               r.Id,
		ExternalChangeId: r.ExternalChangeId,
		Author:           r.Author,
		Bugs:             bugs,
		Description:      r.Description,
		Details:          r.Details,
		Display:          r.Display,
		Dependencies:     util.CopyStringMap(r.Dependencies),
		InvalidReason:    r.InvalidReason,
		Tests:            util.CopyStringSlice(r.Tests),
		Timestamp:        r.Timestamp,
		URL:              r.URL,
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
func FromLongCommit(revLinkTmpl, defaultBugProject string, c *vcsinfo.LongCommit) *Revision {
	revLink := ""
	if revLinkTmpl != "" {
		revLink = fmt.Sprintf(revLinkTmpl, c.Hash)
	}
	author := c.Author
	authorSplit := strings.Split(c.Author, "(")
	if len(authorSplit) > 1 {
		author = strings.TrimRight(strings.TrimSpace(authorSplit[1]), ")")
	}
	return &Revision{
		Id:          c.Hash,
		Author:      author,
		Bugs:        bugsFromCommitMsg(c.Body, defaultBugProject),
		Description: c.Subject,
		Details:     c.Body,
		Display:     c.Hash[:12],
		Tests:       parseTests(c.Body),
		Timestamp:   c.Timestamp,
		URL:         revLink,
	}
}

// FromLongCommits converts a slice of vcsinfo.LongCommits to a slice of
// Revisions. If revLinkTmpl is not provided, the Revisions will have no URL.
func FromLongCommits(revLinkTmpl, defaultBugProject string, commits []*vcsinfo.LongCommit) []*Revision {
	rv := make([]*Revision, 0, len(commits))
	for _, c := range commits {
		rv = append(rv, FromLongCommit(revLinkTmpl, defaultBugProject, c))
	}
	return rv
}

// parseTests parses tests from the Revision details.
func parseTests(details string) []string {
	var tests []string
	for _, match := range testsRe.FindAllString(details, -1) {
		tests = append(tests, match)
	}
	return tests
}

// bugsFromCommitMsg parses BUG= tags from a commit message and returns them.
func bugsFromCommitMsg(msg, defaultBugProject string) map[string][]string {
	rv := map[string][]string{}
	for _, line := range strings.Split(msg, "\n") {
		m := bugsRegex.FindAllStringSubmatch(line, -1)
		for _, match := range m {
			for _, s := range match[1:] {
				for _, field := range strings.Fields(s) {
					bugs := strings.Split(field, ",")
					for _, b := range bugs {
						b = strings.TrimSpace(b)
						split := strings.SplitN(strings.Trim(b, " "), ":", 2)
						project := defaultBugProject
						bug := split[0]
						if len(split) > 1 {
							project = split[0]
							bug = split[1]
						} else if strings.HasPrefix(bug, "b/") {
							project = BugProjectBuganizer
							bug = strings.TrimPrefix(bug, "b/")
						}
						if bug != "" {
							rv[project] = append(rv[project], bug)
						}
					}
				}
			}
		}
	}
	return rv
}
