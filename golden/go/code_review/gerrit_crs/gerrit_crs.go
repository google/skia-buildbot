package gerrit_crs

import (
	"context"
	"errors"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/golden/go/code_review"
)

type CRSImpl struct {
	gClient *gerrit.Gerrit
}

func New(client *gerrit.Gerrit) *CRSImpl {
	return &CRSImpl{
		gClient: client,
	}
}

// GetChangeList implements the code_review.Client interface.
func (c *CRSImpl) GetChangeList(ctx context.Context, id string) (code_review.ChangeList, error) {
	return code_review.ChangeList{}, errors.New("not impl")
}

// GetPatchSets implements the code_review.Client interface.
func (c *CRSImpl) GetPatchSets(ctx context.Context, clID string) ([]code_review.PatchSet, error) {
	return []code_review.PatchSet{}, errors.New("not impl")
}

// GetChangeListForCommit implements the code_review.Client interface.
func (c *CRSImpl) GetChangeListForCommit(ctx context.Context, hash string) (code_review.ChangeList, error) {
	return code_review.ChangeList{}, errors.New("not impl")
}

// Make sure CRSImpl fulfills the code_review.Client interface.
var _ code_review.Client = (*CRSImpl)(nil)
