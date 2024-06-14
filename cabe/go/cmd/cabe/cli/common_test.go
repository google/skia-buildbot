package cli

import (
	"testing"

	cpb "go.skia.org/infra/cabe/go/proto"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/stretchr/testify/assert"
)

func Test_commonCmd_flags(t *testing.T) {
	for _, test := range []struct {
		name       string
		cCmd       commonCmd
		wantLength int
	}{
		{
			name:       "empty",
			wantLength: 5,
		}, {
			name: "complete input",
			cCmd: commonCmd{
				pinpointJobID: "testPinpointJobID",
				recordToZip:   "testRecordToZip",
				replayFromZip: "testReplayFromZip",
				benchmark:     "testBenchmark",
				workloads:     []string{"testWorkload1", "testWorkload2"},
			},
			wantLength: 5,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := test.cCmd.flags()
			assert.Equal(t, test.wantLength, len(got))
		})
	}
}

func Test_commonCmd_experimentSpecFromFlags(t *testing.T) {
	for _, test := range []struct {
		name     string
		cCmd     commonCmd
		wantSpec *cpb.ExperimentSpec
	}{
		{
			name:     "empty",
			wantSpec: nil,
		}, {
			name: "complete input",
			cCmd: commonCmd{
				pinpointJobID: "testPinpointJobID",
				recordToZip:   "testRecordToZip",
				replayFromZip: "testReplayFromZip",
				benchmark:     "testBenchmark",
				workloads:     []string{"testWorkload1", "testWorkload2"},
			},
			wantSpec: &cpb.ExperimentSpec{
				Analysis: &cpb.AnalysisSpec{
					Benchmark: []*cpb.Benchmark{
						{
							Name:     "testBenchmark",
							Workload: []string{"testWorkload1", "testWorkload2"},
						},
					},
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := test.cCmd.experimentSpecFromFlags()
			diff := cmp.Diff(test.wantSpec, got,
				cmpopts.EquateEmpty(),
				protocmp.Transform())
			assert.Equal(t, "", diff)
		})
	}
}
