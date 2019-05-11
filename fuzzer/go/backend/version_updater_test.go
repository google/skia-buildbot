package backend

import (
	"fmt"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/tests"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/testutils/unittest"
)

var ctx = mock.AnythingOfType("*context.emptyCtx")
var callback = mock.AnythingOfType("func(*storage.ObjectAttrs)")

func TestReportWorkDone(t *testing.T) {
	unittest.SmallTest(t)
	mg := tests.NewMockGCSClient()
	mc := tests.NewMockCommonImpl()
	defer mc.AssertExpectations(t)
	defer mg.AssertExpectations(t)

	mc.On("Hostname").Return("skia-fuzzer-be-3").Once()
	common.SetMockCommon(mc)

	// The nil arguments shouldn't be needed in reportWorkDone
	v := NewVersionUpdater(mg, nil, nil)

	mg.On("DeleteFile", ctx, "skia_version/pending/working_skia-fuzzer-be-3").Return(nil).Once()
	mg.On("AllFilesInDirectory", ctx, "skia_version/pending/working_", callback).Run(func(args mock.Arguments) {
		callbackFn := args.Get(2).(func(*storage.ObjectAttrs))
		// Pretend there are still two files, that is, there are still two backends working
		callbackFn(&storage.ObjectAttrs{Name: "working_skia-fuzzer-be-1"})
		callbackFn(&storage.ObjectAttrs{Name: "working_skia-fuzzer-be-2"})
	}).Return(nil).Once()

	assert.NoError(t, v.reportWorkDone("oldRevision", "newRevision"))
}

func TestReportWorkDoneLastStanding(t *testing.T) {
	unittest.SmallTest(t)
	mg := tests.NewMockGCSClient()
	mc := tests.NewMockCommonImpl()
	defer mc.AssertExpectations(t)
	defer mg.AssertExpectations(t)

	mc.On("Hostname").Return("skia-fuzzer-be-3").Once()
	common.SetMockCommon(mc)

	// The nil arguments shouldn't be needed in reportWorkDone
	v := NewVersionUpdater(mg, nil, nil)

	mg.On("DeleteFile", ctx, "skia_version/pending/working_skia-fuzzer-be-3").Return(nil).Once()
	// Suppose there are no other backend workers left (and thus no files)
	mg.On("AllFilesInDirectory", ctx, "skia_version/pending/working_", callback).Return(nil).Once()

	mg.On("DeleteAllFilesInFolder", "skia_version/pending/", 1).Return(nil).Once()
	mg.On("DeleteAllFilesInFolder", "skia_version/current/", 1).Return(nil).Once()
	mg.On("SetFileContents", ctx, "skia_version/current/newRevision", gcs.FILE_WRITE_OPTS_TEXT, []byte("newRevision")).Return(nil).Once()
	mg.On("SetFileContents", ctx, "skia_version/old/oldRevision", gcs.FILE_WRITE_OPTS_TEXT, []byte("oldRevision")).Return(nil).Once()

	assert.NoError(t, v.reportWorkDone("oldRevision", "newRevision"))
}

func TestReportWorkDone404(t *testing.T) {
	unittest.SmallTest(t)
	mg := tests.NewMockGCSClient()
	mc := tests.NewMockCommonImpl()
	defer mc.AssertExpectations(t)
	defer mg.AssertExpectations(t)

	mc.On("Hostname").Return("skia-fuzzer-be-3").Once()
	common.SetMockCommon(mc)

	// The nil arguments shouldn't be needed in reportWorkDone
	v := NewVersionUpdater(mg, nil, nil)

	err := fmt.Errorf("Got non server error statuscode 404")
	mg.On("DeleteFile", ctx, "skia_version/pending/working_skia-fuzzer-be-3").Return(err).Once()
	mg.On("AllFilesInDirectory", ctx, "skia_version/pending/working_", callback).Run(func(args mock.Arguments) {
		callbackFn := args.Get(2).(func(*storage.ObjectAttrs))
		// Pretend there are still two files, that is, there are still two backends working
		callbackFn(&storage.ObjectAttrs{Name: "working_skia-fuzzer-be-1"})
		callbackFn(&storage.ObjectAttrs{Name: "working_skia-fuzzer-be-2"})
	}).Return(nil).Once()

	assert.NoError(t, v.reportWorkDone("oldRevision", "newRevision"))
}
