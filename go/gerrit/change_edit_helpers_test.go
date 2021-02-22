package gerrit

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestAddChangeId(t *testing.T) {
	unittest.SmallTest(t)

	tests := []struct {
		commitMsg         string
		changeId          string
		expectedCommitMsg string
	}{
		{
			commitMsg:         "One line msg",
			changeId:          "123",
			expectedCommitMsg: "One line msg\n\nChange-Id: 123",
		},
		{
			commitMsg:         "Two line msg\n2nd line",
			changeId:          "1234",
			expectedCommitMsg: "Two line msg\n2nd line\n\nChange-Id: 1234",
		},
		{
			commitMsg:         "Empty footers\n\n",
			changeId:          "12345",
			expectedCommitMsg: "Empty footers\n\nChange-Id: 12345",
		},
		{
			commitMsg:         "Empty footers\n\n\n",
			changeId:          "12345",
			expectedCommitMsg: "Empty footers\n\nChange-Id: 12345",
		},
		{
			commitMsg:         "One footer no new line\n\nBug: skia:123",
			changeId:          "123456",
			expectedCommitMsg: "One footer no new line\n\nBug: skia:123\nChange-Id: 123456",
		},
		{
			commitMsg:         "One footer with new line\n\nBug: skia:123\n",
			changeId:          "1234567",
			expectedCommitMsg: "One footer with new line\n\nBug: skia:123\nChange-Id: 1234567",
		},
		{
			commitMsg:         "Two footers\n\nBug: skia:123\nTested: yes",
			changeId:          "12345678",
			expectedCommitMsg: "Two footers\n\nBug: skia:123\nTested: yes\nChange-Id: 12345678",
		},
	}
	for _, test := range tests {
		require.Equal(t, test.expectedCommitMsg, AddChangeId(test.commitMsg, test.changeId))
	}
}
