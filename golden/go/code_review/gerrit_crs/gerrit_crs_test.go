package gerrit_crs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gerrit/mocks"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/code_review"
)

func TestGetChangeListSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mgi := &mocks.GerritInterface{}
	defer mgi.AssertExpectations(t)

	id := "235460"
	ts := time.Date(2019, time.August, 21, 16, 44, 26, 0, time.UTC)
	gci := getOpenGerritChangeInfo()
	mgi.On("GetIssueProperties", anyctx, int64(235460)).Return(&gci, nil)

	c := New(mgi)

	cl, err := c.GetChangeList(context.Background(), id)
	assert.NoError(t, err)
	assert.Equal(t, code_review.ChangeList{
		SystemID: id,
		Owner:    "test@example.com",
		Status:   code_review.Open,
		Subject:  "[gold] Add more tryjob processing tests",
		Updated:  ts,
	}, cl)
}

func TestGetPatchSetsSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mgi := &mocks.GerritInterface{}
	defer mgi.AssertExpectations(t)

	id := "235460"
	gci := getOpenGerritChangeInfo()
	mgi.On("GetIssueProperties", anyctx, int64(235460)).Return(&gci, nil)

	c := New(mgi)

	xps, err := c.GetPatchSets(context.Background(), id)
	assert.NoError(t, err)
	assert.Equal(t, []code_review.PatchSet{
		{
			SystemID:     "993b807277763b351e72d01e6d65461c4bf57981",
			ChangeListID: id,
			Order:        1,
			GitHash:      "993b807277763b351e72d01e6d65461c4bf57981",
		},
		{
			SystemID:     "4cfd5b1ed4d6938efc61fd127bb4a458198ac620",
			ChangeListID: id,
			Order:        2,
			GitHash:      "4cfd5b1ed4d6938efc61fd127bb4a458198ac620",
		},
		{
			SystemID:     "787d20c0117d455ef28cce925e2bb5302c2254ad",
			ChangeListID: id,
			Order:        3,
			GitHash:      "787d20c0117d455ef28cce925e2bb5302c2254ad",
		},
		{
			SystemID:     "337da6ea3a14fd2899b39d0a60c6828971c0d883",
			ChangeListID: id,
			Order:        4,
			GitHash:      "337da6ea3a14fd2899b39d0a60c6828971c0d883",
		},
	}, xps)
}

var (
	anyctx = mock.AnythingOfType("*context.emptyCtx")
)

// Based on a real-world query for a CL that is open and out for review
// with 4 PatchSets
func getOpenGerritChangeInfo() gerrit.ChangeInfo {
	xps := getOpenGerritPatchsets()
	return gerrit.ChangeInfo{
		Id:              "buildbot~master~I29ebaf19a1003e4d9c6df7e5f6469c1f812e0730",
		Created:         time.Date(2019, time.August, 21, 14, 26, 43, 0, time.UTC),
		CreatedString:   "2019-08-21 14:26:43.000000000",
		Updated:         time.Date(2019, time.August, 21, 16, 44, 26, 0, time.UTC),
		UpdatedString:   "2019-08-21 16:44:26.000000000",
		Submitted:       time.Time{},
		SubmittedString: "",
		Project:         "buildbot",
		ChangeId:        "I29ebaf19a1003e4d9c6df7e5f6469c1f812e0730",
		Subject:         "[gold] Add more tryjob processing tests",
		Branch:          "master",
		Committed:       false,
		Revisions: map[string]*gerrit.Revision{
			"337da6ea3a14fd2899b39d0a60c6828971c0d883": xps[3],
			"4cfd5b1ed4d6938efc61fd127bb4a458198ac620": xps[1],
			"787d20c0117d455ef28cce925e2bb5302c2254ad": xps[2],
			"993b807277763b351e72d01e6d65461c4bf57981": xps[0],
		},
		Patchsets:   xps,
		MoreChanges: false,
		Issue:       235460,
		// Labels omitted because it's complex and not needed
		Owner: &gerrit.Owner{
			Email: "test@example.com",
		},
		Status:         "NEW",
		WorkInProgress: false,
	}
}

func getOpenGerritPatchsets() []*gerrit.Revision {
	return []*gerrit.Revision{
		{
			ID:            "993b807277763b351e72d01e6d65461c4bf57981",
			Number:        1,
			CreatedString: "2019-08-21 14:26:43.000000000",
			Created:       time.Date(2019, time.August, 21, 14, 26, 43, 0, time.UTC),
			Kind:          "REWORK",
		},
		{
			ID:            "4cfd5b1ed4d6938efc61fd127bb4a458198ac620",
			Number:        2,
			CreatedString: "2019-08-21 15:28:37.000000000",
			Created:       time.Date(2019, time.August, 21, 15, 28, 37, 0, time.UTC),
			Kind:          "REWORK",
		},
		{
			ID:            "787d20c0117d455ef28cce925e2bb5302c2254ad",
			Number:        3,
			CreatedString: "2019-08-21 16:27:34.000000000",
			Created:       time.Date(2019, time.August, 21, 16, 27, 34, 0, time.UTC),
			Kind:          "REWORK",
		},
		{
			ID:            "337da6ea3a14fd2899b39d0a60c6828971c0d883",
			Number:        4,
			CreatedString: "2019-08-21 16:28:38.000000000",
			Created:       time.Date(2019, time.August, 21, 16, 28, 38, 0, time.UTC),
			Kind:          "NO_CODE_CHANGE",
		},
	}
}
