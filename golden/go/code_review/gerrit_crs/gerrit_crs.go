package gerrit_crs

import (
	"context"
	"errors"
	"sort"
	"strconv"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/code_review"
	"golang.org/x/time/rate"
)

const (
	// These values are arbitrary guesses, roughly based on the values for gitiles.
	maxQPS   = rate.Limit(5.0)
	maxBurst = 20
)

type CRSImpl struct {
	gClient gerrit.GerritInterface
	rl      *rate.Limiter
}

func New(client gerrit.GerritInterface) *CRSImpl {
	return &CRSImpl{
		gClient: client,
		rl:      rate.NewLimiter(maxQPS, maxBurst),
	}
}

var invalidID = errors.New("invalid id - must be integer")

// GetChangeList implements the code_review.Client interface.
func (c *CRSImpl) GetChangeList(ctx context.Context, id string) (code_review.ChangeList, error) {
	i, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return code_review.ChangeList{}, invalidID
	}
	return c.getCL(ctx, i)
}

// getCL fetches a CL from gerrit and converts it into a code_review.ChangeList
func (c *CRSImpl) getCL(ctx context.Context, id int64) (code_review.ChangeList, error) {
	// Respect the rate limit.
	if err := c.rl.Wait(ctx); err != nil {
		return code_review.ChangeList{}, skerr.Wrap(err)
	}
	cl, err := c.gClient.GetIssueProperties(ctx, id)
	if err == gerrit.ErrNotFound {
		return code_review.ChangeList{}, code_review.ErrNotFound
	}
	if err != nil {
		return code_review.ChangeList{}, skerr.Wrapf(err, "fetching CL from gerrit with id %d", id)
	}
	return code_review.ChangeList{
		SystemID: strconv.FormatInt(cl.Issue, 10),
		Owner:    cl.Owner.Email,
		Status:   statusToEnum(cl.Status),
		Subject:  cl.Subject,
		Updated:  cl.Updated,
	}, nil
}

// statusToEnum converts a gerrit status string into a CLStatus enum.
func statusToEnum(g string) code_review.CLStatus {
	switch g {
	case gerrit.CHANGE_STATUS_NEW:
		return code_review.Open
	case gerrit.CHANGE_STATUS_ABANDONED:
		return code_review.Abandoned
	case gerrit.CHANGE_STATUS_MERGED:
		return code_review.Landed
	}
	return code_review.Open
}

// GetPatchSets implements the code_review.Client interface.
func (c *CRSImpl) GetPatchSets(ctx context.Context, clID string) ([]code_review.PatchSet, error) {
	i, err := strconv.ParseInt(clID, 10, 64)
	if err != nil {
		return nil, invalidID
	}
	// Respect the rate limit.
	if err := c.rl.Wait(ctx); err != nil {
		return nil, skerr.Wrap(err)
	}
	cl, err := c.gClient.GetIssueProperties(ctx, i)
	if err == gerrit.ErrNotFound {
		return nil, code_review.ErrNotFound
	}
	if err != nil {
		return nil, skerr.Wrapf(err, "fetching patchsets for CL from gerrit with id %d", i)
	}
	var xps []code_review.PatchSet
	for _, p := range cl.Patchsets {
		xps = append(xps, code_review.PatchSet{
			SystemID:     p.ID,
			ChangeListID: clID,
			Order:        int(p.Number),
			GitHash:      p.ID,
		})
	}
	// Gerrit probably returns them in order, but this ensures it.
	sort.Slice(xps, func(i, j int) bool {
		return xps[i].Order < xps[j].Order
	})

	return xps, nil
}

// GetChangeListForCommit implements the code_review.Client interface.
func (c *CRSImpl) GetChangeListForCommit(ctx context.Context, commit *vcsinfo.LongCommit) (code_review.ChangeList, error) {
	if commit == nil {
		return code_review.ChangeList{}, skerr.Fmt("commit cannot be nil")
	}
	i, err := c.gClient.ExtractIssueFromCommit(commit.Body)
	if err != nil {
		return code_review.ChangeList{}, skerr.Wrapf(err, "finding gerrit cl in %s", commit.Body)
	}

	return c.getCL(ctx, i)
}

// Make sure CRSImpl fulfills the code_review.Client interface.
var _ code_review.Client = (*CRSImpl)(nil)
