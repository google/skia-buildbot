package notify

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/ui/frame"
)

var (
	// Actual shortcut and offset so that the generated URL works against
	// perf.skia.org.
	clusterSummary = &clustering2.ClusterSummary{
		Shortcut: "X70f1ebd38105f4a08d8035d6b283be38",
		StepPoint: &dataframe.ColumnHeader{
			Offset: 68229,
		},
	}
)

func TestViewOnDashboard_HappyPath(t *testing.T) {
	frame := &frame.FrameResponse{
		DataFrame: &dataframe.DataFrame{
			Header: []*dataframe.ColumnHeader{
				{Offset: 1, Timestamp: 0},
				// This timestamp + 1s should map to the 'end' in the query
				// parameters. It is also a valid timestamp for the above
				// clusterSummary so that it generates a working perf.skia.org
				// URL.
				{Offset: 2, Timestamp: 1693815729},
			},
		},
	}
	require.Equal(t, "https://perf.skia.org/e/?end=1693815730&keys=X70f1ebd38105f4a08d8035d6b283be38&num_commits=250&request_type=1&xbaroffset=68229", viewOnDashboard(clusterSummary, "https://perf.skia.org/", frame))
}

func TestViewOnDashboard_FrameIsNil_ReturnsURLWithoutAnEndParam(t *testing.T) {
	var frame *frame.FrameResponse = nil
	require.Equal(t, "https://perf.skia.org/e/?keys=X70f1ebd38105f4a08d8035d6b283be38&num_commits=250&request_type=1&xbaroffset=68229", viewOnDashboard(clusterSummary, "https://perf.skia.org/", frame))
}
