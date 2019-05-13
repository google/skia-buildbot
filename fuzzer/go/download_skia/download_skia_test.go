package download_skia

import (
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/fuzzer/go/tests"
	"go.skia.org/infra/go/testutils/unittest"
)

var ctx = mock.AnythingOfType("*context.emptyCtx")
var callback = mock.AnythingOfType("func(*storage.ObjectAttrs)")

func TestRevisionHelper(t *testing.T) {
	// Tests that we are not dependent on the order the files in the pending or current
	// version, especially when there are working_ tracker files.
	unittest.SmallTest(t)
	m := tests.NewMockGCSClient()
	defer m.AssertExpectations(t)

	expected_rev := "2c65d5161260f3d45a63dcd92229bd09c8a12d53"
	expected_ts := time.Date(2017, time.March, 11, 15, 45, 0, 0, time.UTC)

	m.On("AllFilesInDirectory", ctx, "skia_version/pending/", callback).Run(func(args mock.Arguments) {
		callbackFn := args.Get(2).(func(*storage.ObjectAttrs))
		// Pretend there are still three files, that is, there are still two backends working and the hash
		callbackFn(&storage.ObjectAttrs{Name: "skia_version/pending/working_skia-fuzzer-be-2"})
		callbackFn(&storage.ObjectAttrs{Name: "skia_version/pending/" + expected_rev, Updated: expected_ts})
		callbackFn(&storage.ObjectAttrs{Name: "skia_version/pending/working_skia-fuzzer-be-1"})
		// The folder is sometimes returned as an item
		callbackFn(&storage.ObjectAttrs{Name: "skia_version/pending/"})

	}).Return(nil).Once()

	revision, ts, err := revisionHelper(m, "skia_version/pending/")
	assert.NoError(t, err)
	assert.Equal(t, expected_rev, revision)
	assert.Equal(t, expected_ts, ts)
}
