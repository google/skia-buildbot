package verifiers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	cr_mocks "go.skia.org/infra/skcq/go/codereview/mocks"
	"go.skia.org/infra/skcq/go/types"
)

var (
	authorsContentWithComments = `
# Comment 1
# Comment 2
Batman <batman@gotham.com>
#### Comment 3
Joker <joker@hahaha.com>
Pengiun <pengiun@birds.com>
Riddlers <*@riddleme.com>
`

	authorsMalformedContent = `

# Comment 1
Riddler <.@riddleme.com>
Wrote comment over here
Batman <batman@gotham.com>
	`
)

func TestVerify_Authors(t *testing.T) {
	unittest.SmallTest(t)

	tests := []struct {
		authorsFileContent string
		changeAuthor       string
		expectedState      types.VerifierState
	}{
		{authorsFileContent: "", changeAuthor: "", expectedState: types.VerifierFailureState},
		{authorsFileContent: "", changeAuthor: "batman@gotham.com", expectedState: types.VerifierFailureState},
		{authorsFileContent: "Batman <batman@gotham.com>", changeAuthor: "batman@gotham.com", expectedState: types.VerifierSuccessState},
		{authorsFileContent: "Gotham Citizens <*@gotham.com>", changeAuthor: "batman@gotham.com", expectedState: types.VerifierSuccessState},
		{authorsFileContent: "N <n@gotham.com>", changeAuthor: "batman@gotham.com", expectedState: types.VerifierFailureState},

		{authorsFileContent: authorsContentWithComments, changeAuthor: "batman@gotham.com", expectedState: types.VerifierSuccessState},
		{authorsFileContent: authorsContentWithComments, changeAuthor: "joker@hahaha.com", expectedState: types.VerifierSuccessState},
		{authorsFileContent: authorsContentWithComments, changeAuthor: "r@hahaha.com", expectedState: types.VerifierFailureState},
		{authorsFileContent: authorsContentWithComments, changeAuthor: "riddler@riddleme.com", expectedState: types.VerifierSuccessState},
		{authorsFileContent: authorsContentWithComments, changeAuthor: "riddler@gotham.com", expectedState: types.VerifierFailureState},

		{authorsFileContent: authorsMalformedContent, changeAuthor: "batman@gotham.com", expectedState: types.VerifierSuccessState},
		{authorsFileContent: authorsMalformedContent, changeAuthor: "riddler@riddleme.com", expectedState: types.VerifierFailureState},
	}
	for _, test := range tests {
		// Instantiate test change.
		ci := &gerrit.ChangeInfo{Issue: 123}
		// Setup codereview mock.
		cr := &cr_mocks.CodeReview{}
		cr.On("GetCommitAuthor", testutils.AnyContext, int64(123), "current").Return(test.changeAuthor, nil).Once()

		av := AuthorsVerifier{
			test.authorsFileContent,
			cr,
		}
		state, _, err := av.Verify(context.Background(), ci, int64(333))
		require.NoError(t, err)
		require.Equal(t, test.expectedState, state)
	}
}
