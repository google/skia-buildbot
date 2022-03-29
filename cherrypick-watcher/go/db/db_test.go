package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/firestore/testutils"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
)

const (
	testDBKey     = "test_key"
	testChangeNum = int64(123)
)

func TestGetKey(t *testing.T) {
	unittest.SmallTest(t)

	tests := []struct {
		sourceRepo   string
		sourceBranch string
		targetRepo   string
		targetBranch string
		changeNum    int64
		expectedKey  string
	}{
		{
			sourceRepo:   "skia",
			sourceBranch: "chrome/m100",
			targetRepo:   "skia",
			targetBranch: "android/next-release",
			changeNum:    1000,
			expectedKey:  "skia-chrome-m100_skia-android-next-release_1000",
		},
		{
			sourceRepo:   "skia",
			sourceBranch: "chrome",
			targetRepo:   "skiabot",
			targetBranch: "test1/test2/test3",
			changeNum:    2000,
			expectedKey:  "skia-chrome_skiabot-test1-test2-test3_2000",
		},
	}

	for _, test := range tests {
		require.Equal(t, test.expectedKey, GetKey(test.sourceRepo, test.sourceBranch, test.targetRepo, test.targetBranch, test.changeNum))
	}
}

// newDBClientForTesting returns a FirestoreDB client and ensures that it will
// connect to the Firestore emulator. The Client's instance name will be
// randomized to ensure concurrent tests don't interfere with each other. It also
// returns a CleanupFunc that closes the Client.
func newDBClientForTesting(ctx context.Context, t sktest.TestingT) (*FirestoreDB, util.CleanupFunc) {
	c, cleanup := testutils.NewClientForTesting(ctx, t)
	return &FirestoreDB{
		client: c,
	}, cleanup
}

func TestGetFromDB_EmptyResults_NoErrors(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db, cleanup := newDBClientForTesting(ctx, t)
	defer cleanup()

	// DB should be empty.
	cherrypickData, err := db.GetFromDB(ctx, testDBKey)
	require.Nil(t, cherrypickData)
	require.Nil(t, err)
}

func TestPutInDB_OneEntry_ExpectedChangeNum(t *testing.T) {
	unittest.LargeTest(t)
	db, cleanup := newDBClientForTesting(context.Background(), t)
	defer cleanup()
	ctx := context.Background()

	// DB should be empty. Add one entry.
	err := db.PutInDB(ctx, testDBKey, testChangeNum)
	require.Nil(t, err)

	// Get the added entry and assert.
	cherrypickData, err := db.GetFromDB(ctx, testDBKey)
	require.Equal(t, testChangeNum, cherrypickData.ChangeNumber)
	require.Nil(t, err)
}
