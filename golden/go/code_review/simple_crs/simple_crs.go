// simple_crs is an implementation of code_review.Client that doesn't actually validate anything
// and just pretends any request made to it is valid. This is a workaround for Code Review Systems
// that we have not gotten authentication working yet.
package simple_crs

import (
	"bufio"
	"context"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/code_review"
	"regexp"
	"strings"
	"time"
)

type CRSImpl struct {
	clRegex *regexp.Regexp
}

// New returns a simple CRS implementation that has stubbed calls everywhere where possible.
// If a regex is passed in, it is used to extract the CL ID from the commit message.
func New(clRegex *regexp.Regexp) *CRSImpl {
	return &CRSImpl{
		clRegex: clRegex,
	}
}

// GetChangelist pretends that the requested CL does exist and returns a filled out struct with
// as much data as possible.
func (c *CRSImpl) GetChangelist(_ context.Context, clID string) (code_review.Changelist, error) {
	return code_review.Changelist{
		SystemID: clID,
		Owner:    "Not Available",
		Status:   code_review.Open,
		Subject:  "Not Available",
		Updated:  time.Now(),
	}, nil
}

func (c *CRSImpl) GetPatchset(_ context.Context, clID, psID string, psOrder int) (code_review.Patchset, error) {
	return code_review.Patchset{
		SystemID:     psID,
		ChangelistID: clID,
		Order:        psOrder,
		GitHash:      psID,
	}, nil
}

// GetChangelistIDForCommit returns the result of looking at the commit message using the regex
// provided in the constructor. It is based off Gerrit.ExtractIssueFromCommit because that is the
// current version we want to stub out.
func (c *CRSImpl) GetChangelistIDForCommit(_ context.Context, lc *vcsinfo.LongCommit) (string, error) {
	if c.clRegex == nil {
		return "", skerr.Fmt("Regex is nil")
	}
	scanner := bufio.NewScanner(strings.NewReader(lc.Body))
	for scanner.Scan() {
		line := scanner.Text()
		// Reminder, this regex has the review url (e.g. skia-review.googlesource.com) baked into it.
		result := c.clRegex.FindStringSubmatch(line)
		if len(result) == 2 {
			return result[1], nil
		}
	}
	return "", skerr.Fmt("unable to find Reviewed-on line")
}

func (c *CRSImpl) CommentOn(_ context.Context, clID, _ string) error {
	sklog.Infof("Simple Internal CRS cannot comment on Cl %s, but returns nil error", clID)
	return nil
}

// Make sure CRSImpl fulfills the code_review.Client interface.
var _ code_review.Client = (*CRSImpl)(nil)
