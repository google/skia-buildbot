package analyzer

import (
	"context"
	"fmt"
	"testing"

	swarming "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/cabe/go/perfresults"
	cpb "go.skia.org/infra/cabe/go/proto"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestRun(t *testing.T) {
	ctx := context.Background()

	type runTest struct {
		name                             string
		pinpointID                       []string
		resultsForDigests                map[string]map[string]perfresults.PerfResults
		controlDigests, treatmentDigests []*swarming.SwarmingRpcsCASReference
		expected                         []RResult
		expectedProtos                   []*cpb.AnalysisResult
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
				func(ctx context.Context, instance, digest string) (map[string]perfresults.PerfResults, error) {
					ret, ok := test.resultsForDigests[instance+"/"+digest]
					if !ok {
						return nil, fmt.Errorf("missing instance %q digest %q", instance, digest)
					}
					return ret, nil
				},
			),
			WithSwarmingTaskReader(func(context.Context) ([]*swarming.SwarmingRpcsTaskRequestMetadata, error) { return nil, nil }),
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
