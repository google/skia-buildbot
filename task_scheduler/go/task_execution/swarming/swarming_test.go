package swarming

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.chromium.org/luci/common/api/swarming/swarming/v1"

	"go.skia.org/infra/go/now"
	infra_swarming "go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/swarming/mocks"
	"go.skia.org/infra/go/testutils"
)

func TestGetPendingTasks_LimitsSearchRange(t *testing.T) {

	const testPool = "TEST_POOL"
	fakeNow := time.Date(2022, 04, 3, 2, 1, 0, 0, time.UTC)

	assertStartBeforeNow := func(args mock.Arguments) {
		actualStartTime := args.Get(1).(time.Time)
		assert.True(t, actualStartTime.Before(fakeNow))
	}

	msc := &mocks.ApiClient{}
	msc.On("ListTaskResults", testutils.AnyContext, mock.Anything, fakeNow,
		[]string{"pool:TEST_POOL"}, infra_swarming.TASK_STATE_PENDING, false).
		Run(assertStartBeforeNow).
		Return([]*swarming.SwarmingRpcsTaskResult{}, nil)

	ste := &SwarmingTaskExecutor{
		swarming: msc,
	}

	results, err := ste.GetPendingTasks(now.TimeTravelingContext(fakeNow), testPool)
	require.NoError(t, err)
	assert.Empty(t, results)
}
