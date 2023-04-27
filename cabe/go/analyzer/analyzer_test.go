package analyzer

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	apb "go.skia.org/infra/cabe/go/proto"
	"google.golang.org/protobuf/testing/protocmp"

	swarming "go.chromium.org/luci/common/api/swarming/swarming/v1"
)

func TestRun(t *testing.T) {
	ctx := context.Background()

	type runTest struct {
		name                             string
		pinpointID                       []string
		resultsForDigests                map[string]map[string]PerfResults
		controlDigests, treatmentDigests []*swarming.SwarmingRpcsCASReference
		taskRequestsForPinpoint          map[string][]*swarming.SwarmingRpcsTaskRequest
		taskResultsForPinpoint           map[string][]*swarming.SwarmingRpcsTaskResult
		expected                         []RResult
		expectedProtos                   []*apb.AnalysisResult
		expectError                      bool
	}

	for i, test := range []runTest{
		{
			name:        "empty",
			pinpointID:  []string{""},
			expectError: true,
		},
	} {
		e := New(
			WithCASResultReader(
				func(ctx context.Context, instance, digest string) (map[string]PerfResults, error) {
					ret, ok := test.resultsForDigests[instance+"/"+digest]
					if !ok {
						return nil, fmt.Errorf("missing instance %q digest %q", instance, digest)
					}
					return ret, nil
				},
			),
			WithTaskRequestsReader(
				func(context.Context) ([]*swarming.SwarmingRpcsTaskRequest, error) {
					ret, ok := test.taskRequestsForPinpoint[test.pinpointID[0]]
					if !ok {
						return nil, fmt.Errorf("missing task request id %s", test.pinpointID[0])
					}
					return ret, nil
				},
			),
			WithTaskResultsReader(
				func(context.Context) ([]*swarming.SwarmingRpcsTaskResult, error) {
					ret, ok := test.taskResultsForPinpoint[test.pinpointID[0]]
					if !ok {
						return nil, fmt.Errorf("missing task result id %s", test.pinpointID[0])
					}
					return ret, nil
				},
			),
		)

		err := e.Run(ctx)
		if err != nil && !test.expectError {
			t.Errorf("[%d] %q unexpected error: %v", i, test.name, err)
		}
		if err == nil && test.expectError {
			t.Errorf("[%d] %q did not return expected error", i, test.name)
		}

		res := e.Results()
		if len(res) != len(test.expected) {
			t.Errorf("[%d] %q expected %d results, got %d", i, test.name, len(test.expected), len(res))
		}

		diff := cmp.Diff(test.expected, res, cmpopts.EquateEmpty(), cmpopts.EquateApprox(0, 0.03))
		if diff != "" {
			t.Errorf("[%d] %q results didn't match expected value. Diff:\n%s", i, test.name, diff)
		}

		// Don't bother trying to process the results further or check other assumptions about the state
		// of e, if this test case is looking for an error.
		if test.expectError {
			continue
		}

		protoRes := e.AnalysisResults()
		if diff := cmp.Diff(test.expectedProtos, protoRes,
			cmpopts.EquateEmpty(),
			cmpopts.EquateApprox(0, 0.03),
			protocmp.Transform()); diff != "" {
			t.Errorf("[%d] %q result proto didn't match expected value. Diff:\n%s", i, test.name, diff)
		}
	}
}
