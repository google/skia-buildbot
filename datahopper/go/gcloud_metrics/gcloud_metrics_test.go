package gcloud_metrics

import (
	"context"
	"testing"
	"time"

	"github.com/googleapis/gax-go/v2"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/testutils/unittest"
	"google.golang.org/api/iterator"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// testMetricClient implements MetricClient for testing.
type testMetricClient struct {
	mockedTimeSeries []*monitoringpb.TimeSeries
}

func (c *testMetricClient) ListTimeSeries(context.Context, *monitoringpb.ListTimeSeriesRequest, ...gax.CallOption) TimeSeriesIterator {
	return &testTimeSeriesIterator{
		timeSeries: c.mockedTimeSeries,
		idx:        0,
	}
}

type testTimeSeriesIterator struct {
	timeSeries []*monitoringpb.TimeSeries
	idx        int
}

func (i *testTimeSeriesIterator) Next() (*monitoringpb.TimeSeries, error) {
	if i.idx >= len(i.timeSeries) {
		return nil, iterator.Done
	}
	rv := i.timeSeries[i.idx]
	i.idx++
	return rv, nil
}

func makeTimeSeries(labels map[string]string, points []int64) *monitoringpb.TimeSeries {
	pointsPb := make([]*monitoringpb.Point, 0, len(points))
	for _, point := range points {
		pointsPb = append(pointsPb, &monitoringpb.Point{
			Value: &monitoringpb.TypedValue{
				Value: &monitoringpb.TypedValue_Int64Value{
					Int64Value: point,
				},
			},
		})
	}
	return &monitoringpb.TimeSeries{
		Points: pointsPb,
		Resource: &monitoredres.MonitoredResource{
			Labels: labels,
		},
	}
}

func TestIngestTimeSeries(t *testing.T) {
	unittest.SmallTest(t)

	// Set up mocks.
	ctx := context.Background()
	metricsClient := &testMetricClient{
		// We use two time series with different values and slightly different
		// label sets to verify that ingestTimeSeries finds the correct metrics.
		mockedTimeSeries: []*monitoringpb.TimeSeries{
			makeTimeSeries(map[string]string{
				"key1": "value1a",
				"key2": "value2a",
				"key3": "value3a",
			}, []int64{9, 10, 11}),
			makeTimeSeries(map[string]string{
				"key2": "value2b",
				"key3": "value3b",
				"key4": "value4b",
			}, []int64{3, 22, 21}),
		},
	}
	// Timestamps are ignored by the mock client.
	startTime := timestamppb.New(time.Time{})
	endTime := timestamppb.New(time.Time{})
	measurement := "fake_measurement"
	project := "fake-project"
	metrics, err := ingestTimeSeries(ctx, metricsClient, project, "fake-metric-type", measurement, []string{"key1", "key2"}, startTime, endTime)
	require.NoError(t, err)
	require.Len(t, metrics, 2)

	// Ensure that we registered the metrics as expected.
	m0 := metrics2.GetInt64Metric(measurement, map[string]string{
		"key1":    "value1a",
		"key2":    "value2a",
		"project": project, // Added automatically by ingestTimeSeries.
	})
	require.Equal(t, m0, metrics[0])
	require.Equal(t, int64(11), m0.Get()) // We only use the last data point.
	m1 := metrics2.GetInt64Metric(measurement, map[string]string{
		"key1":    "", // Not present in the original time series.
		"key2":    "value2b",
		"project": project, // Added automatically by ingestTimeSeries.
	})
	require.Equal(t, m1, metrics[1])
	require.Equal(t, int64(21), m1.Get()) // We only use the last data point.
}

var _ MetricClient = &testMetricClient{}
