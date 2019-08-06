package fs_baseliner

import (
	"testing"

	"go.skia.org/infra/go/testutils/unittest"
)

// import (
// 	"testing"

// 	"github.com/stretchr/testify/assert"
// 	"go.skia.org/infra/go/testutils/unittest"
// 	"go.skia.org/infra/golden/go/mocks"
// 	three_devices "go.skia.org/infra/golden/go/testutils/data_three_devices"
// 	"go.skia.org/infra/golden/go/types"
// )

// // Test that the baseliner produces a baseline
// func TestFetchBaselineSunnyDay(t *testing.T) {
// 	unittest.SmallTest(t)

// 	testCommitHash := "abcd12345"

// 	mes := mocks.ExpectationsStore{}
// 	defer mes.AssertExpectations(t)

// 	mes.On("Get").Return(three_devices.MakeTestExpectations(), nil).Once()

// 	baseliner := New(mes)

// 	b, err := baseliner.FetchBaseline(testCommitHash, types.MasterBranch, false)
// 	assert.NoError(t, err)

// 	expectedBaseline := three_devices.MakeTestExpectations().AsBaseline()

// 	assert.Equal(t, expectedBaseline, b.Expectations)
// 	assert.Equal(t, types.MasterBranch, b.Issue)
// }

func TestHello(t *testing.T) {
	unittest.SmallTest(t)
}
