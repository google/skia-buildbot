package frontend

import (
	"testing"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/fuzzer/go/tests"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/testutils/unittest"
)

var ctx = mock.AnythingOfType("*context.emptyCtx")

func TestUpdateVersionToFuzz(t *testing.T) {
	unittest.SmallTest(t)
	m := tests.NewMockGCSClient()
	defer m.AssertExpectations(t)

	testRevision := "abcde12345"

	m.On("SetFileContents", ctx, "skia_version/pending/"+testRevision, gcs.FILE_WRITE_OPTS_TEXT, []byte(testRevision)).Return(nil).Once()
	f := "skia_version/pending/working_skia-fuzzer-be-1"
	m.On("SetFileContents", ctx, f, gcs.FILE_WRITE_OPTS_TEXT, []byte(f)).Return(nil).Once()
	f = "skia_version/pending/working_skia-fuzzer-be-2"
	m.On("SetFileContents", ctx, f, gcs.FILE_WRITE_OPTS_TEXT, []byte(f)).Return(nil).Once()

	assert.NoError(t, UpdateVersionToFuzz(m, []string{"skia-fuzzer-be-1", "skia-fuzzer-be-2"}, testRevision))
}
