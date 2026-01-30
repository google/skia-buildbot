package dfiter

import (
	"context"
	"sort"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/types"
)

// stepFitDfTraceSlicer implements DataFrameIterator by providing a sliding window over
// each trace in a larger DataFrame. It iterates through all windows of one
// trace before moving to the next.
//
// For each trace it filters out the missing data points and creates a dense
// trace of valid data points, and then iterates over that dense data with a
// sliding window.
type stepFitDfTraceSlicer struct {
	keys              []string // Sorted list of trace keys.
	windowSize        int      // The size of the sliding window, e.g., 2*radius+1.
	currentTraceIndex int      // The index into 'keys' for the current trace.
	currentOffset     int      // The starting offset of the window in the current trace.

	// filteredTraces contains the traces with MissingDataSentinel values removed.
	filteredTraces map[string][]float32

	// filteredHeaders contains the headers corresponding to the values in
	// filteredTraces.
	filteredHeaders map[string][]*dataframe.ColumnHeader
}

// See DataFrameIterator.
func (d *stepFitDfTraceSlicer) Next() bool {
	// Create temporary variables so we don't modify state.
	offset := d.currentOffset
	traceIndex := d.currentTraceIndex

	for traceIndex < len(d.keys) {
		traceID := d.keys[traceIndex]
		fullTrace := d.filteredTraces[traceID]

		if offset+d.windowSize <= len(fullTrace) {
			return true
		}

		// Move to the next trace and reset offset.
		traceIndex++
		offset = 0
	}
	return false
}

// See DataFrameIterator.
func (d *stepFitDfTraceSlicer) Value(ctx context.Context) (*dataframe.DataFrame, error) {
	// Advance to the next trace if the current one is exhausted.
	// This loop handles traces that are too short to produce a window.
	for d.currentOffset+d.windowSize > len(d.filteredTraces[d.keys[d.currentTraceIndex]]) {
		d.currentTraceIndex++
		d.currentOffset = 0
	}

	// Get the current trace based on the (potentially advanced) state.
	traceID := d.keys[d.currentTraceIndex]
	fullTrace := d.filteredTraces[traceID]
	fullHeader := d.filteredHeaders[traceID]

	// Define the slice boundaries for the window.
	start := d.currentOffset
	end := d.currentOffset + d.windowSize

	// Create the sliced trace data and header for the window.
	slicedTrace := fullTrace[start:end]
	slicedHeader := fullHeader[start:end]

	// IMPORTANT: Advance the offset for the next call to Value().
	d.currentOffset++

	// Create and return the new DataFrame containing just the window.
	ret := dataframe.NewEmpty()
	ret.Header = slicedHeader
	ret.TraceSet = types.TraceSet{
		traceID: slicedTrace,
	}
	ret.BuildParamSet()

	return ret, nil
}

// NewStepFitDfTraceSlicer creates a new stepFitDfTraceSlicer.
//
// df - The DataFrame to iterate over.
// radius - The radius used to calculate the sliding window size.
func NewStepFitDfTraceSlicer(df *dataframe.DataFrame, radius int) *stepFitDfTraceSlicer {
	keys := make([]string, 0, len(df.TraceSet))
	filteredTraces := map[string][]float32{}
	filteredHeaders := map[string][]*dataframe.ColumnHeader{}
	filteredMetric := metrics2.GetCounter("perf_stepfitdftraceslicer_filtered_points")

	// For each trace filter out missing values.
	for k, trace := range df.TraceSet {
		keys = append(keys, k)

		validPoints := []float32{}
		validHeaders := []*dataframe.ColumnHeader{}
		for i, v := range trace {
			if v != vec32.MissingDataSentinel {
				validPoints = append(validPoints, v)
				validHeaders = append(validHeaders, df.Header[i])
			} else {
				filteredMetric.Inc(1)
			}
		}
		filteredTraces[k] = validPoints
		filteredHeaders[k] = validHeaders
	}

	sort.Strings(keys)
	return &stepFitDfTraceSlicer{
		keys:              keys,
		windowSize:        2*radius + 1,
		currentTraceIndex: 0,
		currentOffset:     0,
		filteredTraces:    filteredTraces,
		filteredHeaders:   filteredHeaders,
	}
}

type kmeansDataframeSlicer struct {
	df     *dataframe.DataFrame
	size   int
	offset int
}

// See DataFrameIterator.
func (d *kmeansDataframeSlicer) Next() bool {
	return d.offset+d.size <= len(d.df.Header)
}

// See DataFrameIterator.
func (d *kmeansDataframeSlicer) Value(ctx context.Context) (*dataframe.DataFrame, error) {
	// Slice off a sub-dataframe from d.df.
	df, err := d.df.Slice(d.offset, d.size)
	if err != nil {
		return nil, err
	}
	d.offset += 1
	return df, nil
}

func NewKmeansDataframeSlicer(df *dataframe.DataFrame, radius int) *kmeansDataframeSlicer {
	return &kmeansDataframeSlicer{
		df:     df,
		size:   2*radius + 1,
		offset: 0,
	}
}
