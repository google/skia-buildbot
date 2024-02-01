package read_values

import (
	"context"
	"fmt"
	"math"

	"go.skia.org/infra/cabe/go/backends"
	"go.skia.org/infra/cabe/go/perfresults"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/exp/slices"

	rbeclient "github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
)

// define data aggregation method enums
type aggDataMethod int

// Performance test results can be aggregated in the following ways.
// When a aggregation method is chosen, all values generated by
// one CAS digest is aggregated.
const (
	// Count is the number of data points
	Count aggDataMethod = iota
	// Max is the maximum of the data sample
	Max
	// Mean is the average of the data sample
	Mean
	// Min is the minimum of the data sample
	Min
	// Std is the standard deviation of the data sample
	Std
	// Sum is the sum of the data sample
	Sum
)

type AggDataMethodEnum interface {
	AggDataMethod() aggDataMethod
}

func (a aggDataMethod) AggDataMethod() aggDataMethod {
	return a
}

// DialRBECAS dials an RBE CAS client given a swarming instance.
// Pinpoint uses 3 swarming instances to store CAS results
// https://skia.googlesource.com/buildbot/+/5291743c698e/cabe/go/backends/rbecas.go#19
func DialRBECAS(ctx context.Context, instance string) (*rbeclient.Client, error) {
	clients, err := backends.DialRBECAS(ctx)
	if err != nil {
		sklog.Errorf("Failed to dial RBE CAS client due to error: %v", err)
		return nil, err
	}
	if client, ok := clients[instance]; ok {
		return client, nil
	}
	return nil, fmt.Errorf("Swarming instance %s is not within the set of allowed instances", instance)
}

// ReadValuesByChart reads Pinpoint results for specific benchmark and chart from a list of CAS digests.
// ReadValuesByChart will also apply any data aggregations.
//
// Example Usage:
//
//	ctx := context.Background()
//	client, err := DialRBECAS(ctx)
//	values := client.ReadValuesByChart(ctx, client, benchmark, chart, digests, nil)
func ReadValuesByChart(ctx context.Context, client *rbeclient.Client,
	benchmark string, chart string, digests []*swarmingV1.SwarmingRpcsCASReference,
	agg AggDataMethodEnum) ([]float64, error) {
	values := []float64{}
	for _, digest := range digests {
		res, err := backends.FetchBenchmarkJSON(ctx, client,
			fmt.Sprintf("%s/%d", digest.Digest.Hash, digest.Digest.SizeBytes))
		if err != nil {
			return nil, skerr.Wrapf(err,
				"Could not fetch results from CAS %s",
				fmt.Sprintf("%s/%d", digest.Digest.Hash, digest.Digest.SizeBytes),
			)
		}
		v := ReadChart(res, benchmark, chart)
		if agg != nil {
			s, err := aggData(v, agg)
			if err != nil {
				return nil, skerr.Wrapf(err, "Could not aggregate data via %v on %v", agg.AggDataMethod(), v)
			}
			values = append(values, s)
		} else {
			values = append(values, v...)
		}
	}
	return values, nil
}

// ReadChart reads the specific benchmark and chart data from one CAS digest
func ReadChart(data map[string]perfresults.PerfResults, benchmark string, chart string) []float64 {
	var v = []float64{}
	for b := range data {
		if b == benchmark {
			for _, hist := range data[benchmark].Histograms {
				if hist.Name == chart {
					v = append(v, hist.SampleValues...)
				}
			}
		}
	}

	return v
}

func aggData(data []float64, agg AggDataMethodEnum) (float64, error) {
	if data == nil {
		return 0.0, skerr.Fmt("Cannot aggregate nil data set")
	}
	if agg.AggDataMethod() == Count.AggDataMethod() {
		return float64(len(data)), nil
	}
	if len(data) == 0 {
		return 0.0, skerr.Fmt("Empty data set cannot be aggregated by %v", agg.AggDataMethod())
	}
	if agg.AggDataMethod() == Max.AggDataMethod() {
		return slices.Max(data), nil
	} else if agg.AggDataMethod() == Mean.AggDataMethod() {
		return sum(data) / float64(len(data)), nil
	} else if agg.AggDataMethod() == Min.AggDataMethod() {
		return slices.Min(data), nil
	} else if agg.AggDataMethod() == Std.AggDataMethod() {
		return stdDev(data), nil
	} else if agg.AggDataMethod() == Sum.AggDataMethod() {
		return sum(data), nil
	}
	return 0.0, skerr.Fmt("Aggregation method %v is not implemented", agg.AggDataMethod())
}

func sum(data []float64) float64 {
	s := 0.0
	for i := range data {
		s += data[i]
	}
	return s
}

// stdDev returns the sample standard deviation of an array of data
// based off of the implementation here:
// https://source.chromium.org/chromium/chromium/src/+/main:third_party/catapult/tracing/tracing/value/histogram.py;drc=22e558b5843a77389ca3883d0950f0f34e6f690c;l=299
func stdDev(data []float64) float64 {
	sum := sum(data)
	mean := sum / float64(len(data))
	vr := 0.0
	for _, x := range data {
		vr += (x - mean) * (x - mean)
	}
	stddev := math.Sqrt(float64(vr / float64(len(data)-1)))
	return stddev
}