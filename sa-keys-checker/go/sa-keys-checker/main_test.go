package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	adminpb "google.golang.org/genproto/googleapis/iam/admin/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.skia.org/infra/go/metrics2"
)

// testFloat64Metric implements the Float64Metric interface.
type testFloat64Metric struct {
	id          int
	deleteCount int
}

func (m *testFloat64Metric) Delete() error {
	m.deleteCount += 1
	return nil
}

func (m *testFloat64Metric) Get() float64 {
	return 0
}

func (m *testFloat64Metric) Update(v float64) {
	return
}

func TestDeleteUnusedMetrics(t *testing.T) {

	m1 := &testFloat64Metric{id: 1}
	m2 := &testFloat64Metric{id: 2}
	m3 := &testFloat64Metric{id: 3}
	m4 := &testFloat64Metric{id: 4}

	tests := []struct {
		oldMetrics            map[metrics2.Float64Metric]struct{}
		newMetrics            map[metrics2.Float64Metric]struct{}
		m1ExpectedDeleteCount int
		m2ExpectedDeleteCount int
		m3ExpectedDeleteCount int
		m4ExpectedDeleteCount int
	}{
		{
			// Empty old and new metrics.
			oldMetrics:            map[metrics2.Float64Metric]struct{}{},
			newMetrics:            map[metrics2.Float64Metric]struct{}{},
			m1ExpectedDeleteCount: 0,
			m2ExpectedDeleteCount: 0,
			m3ExpectedDeleteCount: 0,
			m4ExpectedDeleteCount: 0,
		},
		{
			// Empty old metrics.
			oldMetrics:            map[metrics2.Float64Metric]struct{}{},
			newMetrics:            map[metrics2.Float64Metric]struct{}{m1: {}, m2: {}, m3: {}},
			m1ExpectedDeleteCount: 0,
			m2ExpectedDeleteCount: 0,
			m3ExpectedDeleteCount: 0,
			m4ExpectedDeleteCount: 0,
		},
		{
			// Old metrics has m1 and m2. New metrics is empty.
			oldMetrics:            map[metrics2.Float64Metric]struct{}{m1: {}, m2: {}},
			newMetrics:            map[metrics2.Float64Metric]struct{}{},
			m1ExpectedDeleteCount: 1,
			m2ExpectedDeleteCount: 1,
			m3ExpectedDeleteCount: 0,
			m4ExpectedDeleteCount: 0,
		},
		{
			// Different metrics in old and new. m1 and m2 should be deleted.
			oldMetrics:            map[metrics2.Float64Metric]struct{}{m1: {}, m2: {}},
			newMetrics:            map[metrics2.Float64Metric]struct{}{m3: {}, m4: {}},
			m1ExpectedDeleteCount: 1,
			m2ExpectedDeleteCount: 1,
			m3ExpectedDeleteCount: 0,
			m4ExpectedDeleteCount: 0,
		},
		{
			// Same m1, m2, m3, m4 metrics in old and new.
			oldMetrics:            map[metrics2.Float64Metric]struct{}{m1: {}, m2: {}, m3: {}, m4: {}},
			newMetrics:            map[metrics2.Float64Metric]struct{}{m1: {}, m2: {}, m3: {}, m4: {}},
			m1ExpectedDeleteCount: 0,
			m2ExpectedDeleteCount: 0,
			m3ExpectedDeleteCount: 0,
			m4ExpectedDeleteCount: 0,
		},
		{
			// Different metrics in old and new with m2 and m3 overlapping.
			oldMetrics:            map[metrics2.Float64Metric]struct{}{m1: {}, m2: {}, m3: {}},
			newMetrics:            map[metrics2.Float64Metric]struct{}{m2: {}, m3: {}, m4: {}},
			m1ExpectedDeleteCount: 1,
			m2ExpectedDeleteCount: 0,
			m3ExpectedDeleteCount: 0,
			m4ExpectedDeleteCount: 0,
		},
	}

	for _, test := range tests {
		deleteUnusedMetrics(test.oldMetrics, test.newMetrics)

		// Assert expected values.
		require.Equal(t, test.m1ExpectedDeleteCount, m1.deleteCount)
		require.Equal(t, test.m2ExpectedDeleteCount, m2.deleteCount)
		require.Equal(t, test.m3ExpectedDeleteCount, m3.deleteCount)
		require.Equal(t, test.m4ExpectedDeleteCount, m4.deleteCount)

		// Reset all counts for the next test.
		m1.deleteCount = 0
		m2.deleteCount = 0
		m3.deleteCount = 0
		m4.deleteCount = 0
	}
}

func TestProcessKey(t *testing.T) {

	tests := []struct {
		key                    *adminpb.ServiceAccountKey
		expectedMetricsMapSize int
	}{
		{
			// Auto-generated key with < 21 days duration should not be added to metrics map.
			key: &adminpb.ServiceAccountKey{
				Name:            "key1",
				ValidBeforeTime: timestamppb.New(time.Now()),
				ValidAfterTime:  timestamppb.New(time.Now().Add(-20 * 24 * time.Hour)),
			},
			expectedMetricsMapSize: 0,
		},
		{
			// Auto-generated key with < 21 days duration should not be added to metrics map.
			key: &adminpb.ServiceAccountKey{
				Name:            "key2",
				ValidBeforeTime: timestamppb.New(time.Now()),
				ValidAfterTime:  timestamppb.New(time.Now().Add(-1 * 24 * time.Hour)),
			},
			expectedMetricsMapSize: 0,
		},
		{
			// Manually generated key that is > 21 days duration should be added to metrics map.
			key: &adminpb.ServiceAccountKey{
				Name:            "key3",
				ValidBeforeTime: timestamppb.New(time.Now()),
				ValidAfterTime:  timestamppb.New(time.Now().Add(-30 * 24 * time.Hour)),
			},
			expectedMetricsMapSize: 1,
		},
	}

	for _, test := range tests {
		metrics := map[metrics2.Float64Metric]struct{}{}
		processKey(test.key, metrics, "sa-name", "project-name")
		require.Len(t, metrics, test.expectedMetricsMapSize)
	}
}
