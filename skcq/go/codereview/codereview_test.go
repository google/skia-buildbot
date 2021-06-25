package codereview

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gerrit/mocks"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/testutils"
)

var (
	// http.Client used for testing.
	c = httputils.NewTimeoutClient()
)

func TestSearch(t *testing.T) {
	g := &mocks.GerritInterface{}
	g.On("Config").Return(gerrit.ConfigChromium)

	// Mock search call to CQ runs.
	cqChangeID1 := int64(123)
	cqChangeInfo1 := &gerrit.ChangeInfo{Issue: cqChangeID1}
	g.On(
		"Search",
		testutils.AnyContext,
		GerritOpenChangesNum,
		true,
		gerrit.SearchStatus(gerrit.ChangeStatusOpen),
		gerrit.SearchLabel(gerrit.LabelCommitQueue, strconv.Itoa(gerrit.LabelCommitQueueSubmit)),
	).Return([]*gerrit.ChangeInfo{cqChangeInfo1}, nil)
	g.On("GetIssueProperties", testutils.AnyContext, cqChangeID1).Return(cqChangeInfo1, nil)

	// Mock search call to dry-runs.
	dryRunChangeID1 := int64(123) // Same ID as cqChangeID1 to test for deduplication.
	dryRunChangeID2 := int64(345)
	dryRunChangeID3 := int64(902)
	dryRunChangeInfo1 := &gerrit.ChangeInfo{Issue: dryRunChangeID1}
	dryRunChangeInfo2 := &gerrit.ChangeInfo{Issue: dryRunChangeID2}
	dryRunChangeInfo3 := &gerrit.ChangeInfo{Issue: dryRunChangeID3}
	g.On(
		"Search",
		testutils.AnyContext,
		GerritOpenChangesNum,
		true,
		gerrit.SearchStatus(gerrit.ChangeStatusOpen),
		gerrit.SearchLabel(gerrit.LabelCommitQueue, strconv.Itoa(gerrit.LabelCommitQueueDryRun)),
	).Return([]*gerrit.ChangeInfo{dryRunChangeInfo1, dryRunChangeInfo2, dryRunChangeInfo3}, nil)

	cr := gerritCodeReview{
		gerritClient: g,
		cfg:          gerrit.ConfigChromium,
	}
	changes, err := cr.Search(context.Background())
	require.NoError(t, err)
	require.Len(t, changes, 3)
	require.True(t, deepequal.DeepEqual([]*gerrit.ChangeInfo{cqChangeInfo1, dryRunChangeInfo2, dryRunChangeInfo3}, changes))
}
