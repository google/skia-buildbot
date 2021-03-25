package gerrit

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gerrit/mocks"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestGetPatchsetInfo_HappyPath(t *testing.T) {
	unittest.SmallTest(t)

	gc := &mocks.GerritInterface{}
	changeInfo := &gerrit.ChangeInfo{
		Status: gerrit.ChangeStatusNew,
		Patchsets: []*gerrit.Revision{
			{
				Ref: "refs/changes/96/386796/22",
			},
		},
	}
	gc.On("GetChange", testutils.AnyContext, "123").Return(changeInfo, nil)

	cr := gerritCodeReview{
		gc: gc,
	}
	ref, isClosed, err := cr.GetPatchsetInfo(context.Background(), "123")
	require.NoError(t, err)
	require.False(t, isClosed)
	require.Equal(t, ref, changeInfo.Patchsets[0].Ref)
}

var myFakeError = fmt.Errorf("My fake error")

func TestGetPatchsetInfo_GetChangeFails_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	gc := &mocks.GerritInterface{}
	gc.On("GetChange", testutils.AnyContext, "123").Return(nil, myFakeError)

	cr := gerritCodeReview{
		gc: gc,
	}
	_, _, err := cr.GetPatchsetInfo(context.Background(), "123")
	require.Contains(t, err.Error(), myFakeError.Error())
}
