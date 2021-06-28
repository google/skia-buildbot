package verifiers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/skcq/go/types"
	"go.skia.org/infra/skcq/go/types/mocks"
)

var (
	vmCurrentTime = time.Unix(1598467386, 0).UTC()
)

func TestRunVerifiers(t *testing.T) {
	ci := &gerrit.ChangeInfo{Issue: int64(123)}
	startTime := vmCurrentTime.Unix() - 100
	reason1 := "verify1 succeeded"
	reason2 := "verify2 is waiting"
	reason3 := "verify3 failed"
	vmTimeNowFunc = func() time.Time {
		return currentTime
	}

	// Setup mock for verifiers.
	v1 := &mocks.Verifier{}
	v1.On("Verify", testutils.AnyContext, ci, startTime).Return(types.VerifierSuccessState, reason1, nil).Once()
	v1.On("Name").Return("verifier1").Once()
	v2 := &mocks.Verifier{}
	v2.On("Verify", testutils.AnyContext, ci, startTime).Return(types.VerifierWaitingState, reason2, nil).Once()
	v2.On("Name").Return("verifier2").Once()
	v3 := &mocks.Verifier{}
	v3.On("Verify", testutils.AnyContext, ci, startTime).Return(types.VerifierFailureState, reason3, nil).Once()
	v3.On("Name").Return("verifier3").Once()
	// v3 will throw a transient error.
	v4 := &mocks.Verifier{}
	v4.On("Verify", testutils.AnyContext, ci, startTime).Return(types.VerifierState(""), "", fmt.Errorf("Error")).Once()
	v4.On("Name").Return("verifier4").Twice()

	vm := &SkCQVerifiersManager{}
	statuses := vm.RunVerifiers(context.Background(), ci, []types.Verifier{v1, v2, v3, v4}, startTime)
	require.Len(t, statuses, 4)
	// Assert the successful verifier.
	require.Equal(t, types.VerifierSuccessState, statuses[0].State)
	require.Equal(t, reason1, statuses[0].Reason)
	require.Equal(t, startTime, statuses[0].Start)
	require.Equal(t, currentTime.Unix(), statuses[0].Stop)
	// Assert the waiting verifier.
	require.Equal(t, types.VerifierWaitingState, statuses[1].State)
	require.Equal(t, reason2, statuses[1].Reason)
	require.Equal(t, startTime, statuses[1].Start)
	require.Equal(t, int64(0), statuses[1].Stop)
	// Assert the failed verifier.
	require.Equal(t, types.VerifierFailureState, statuses[2].State)
	require.Equal(t, reason3, statuses[2].Reason)
	require.Equal(t, startTime, statuses[2].Start)
	require.Equal(t, currentTime.Unix(), statuses[2].Stop)
	// Assert the verifier that had an error
	require.Equal(t, types.VerifierWaitingState, statuses[3].State)
	require.Equal(t, startTime, statuses[3].Start)
	require.Equal(t, int64(0), statuses[3].Stop)
}
