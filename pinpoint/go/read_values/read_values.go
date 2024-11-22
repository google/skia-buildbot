package read_values

import (
	"context"
	"fmt"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/cabe/go/backends"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/perfresults"

	rbeclient "github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
)

// IsSupportedAggregation checks if the aggregation method
// is supported by read_values. If not, return false.
// Empty string is supported and means that no data will be aggregated.
func IsSupportedAggregation(aggregationMethod string) bool {
	if aggregationMethod == "" {
		return true
	}
	if _, ok := perfresults.AggregationMapping[aggregationMethod]; ok {
		return true
	}
	return false
}

// CASProvider provides API to fetch perf results from a given CAS digest.
type CASProvider interface {
	Fetch(context.Context, *apipb.CASReference) (map[string]perfresults.PerfResults, error)
}

// rbeProvider implements CASProvider to fetch perf results from RBE backend.
type rbeProvider struct {
	*rbeclient.Client
}

func (r *rbeProvider) Fetch(ctx context.Context, digest *apipb.CASReference) (map[string]perfresults.PerfResults, error) {
	path := fmt.Sprintf("%s/%d", digest.Digest.Hash, digest.Digest.SizeBytes)
	return backends.FetchBenchmarkJSON(ctx, r.Client, path)
}

type perfCASClient struct {
	provider CASProvider
}

// DialRBECAS dials an RBE CAS client given a swarming instance.
// Pinpoint uses 3 swarming instances to store CAS results
// https://skia.googlesource.com/buildbot/+/5291743c698e/cabe/go/backends/rbecas.go#19
func DialRBECAS(ctx context.Context, instance string) (*perfCASClient, error) {
	clients, err := backends.DialRBECAS(ctx)
	if err != nil {
		sklog.Errorf("Failed to dial RBE CAS client due to error: %v", err)
		return nil, err
	}
	if client, ok := clients[instance]; ok {
		return &perfCASClient{
			provider: &rbeProvider{
				Client: client,
			},
		}, nil
	}
	return nil, fmt.Errorf("swarming instance %s is not within the set of allowed instances", instance)
}

// ReadValuesByChart reads Pinpoint results for the benchmark and chart from a list of CAS digests.
// ReadValuesByChart will also apply data aggregations if there are any.
//
// Example Usage:
//
//	ctx := context.Background()
//	client, err := DialRBECAS(ctx)
//	values := client.ReadValuesByChart(ctx, benchmark, chart, digests, nil)
//
// TODO(sunxiaodi@): Migrate CABE backends into pinpoint/go/backends/
func (c *perfCASClient) ReadValuesByChart(ctx context.Context, benchmark string, chart string, digests []*apipb.CASReference, agg string) ([]float64, error) {
	aggMethod, ok := perfresults.AggregationMapping[agg]
	if !ok && agg != "" {
		return nil, skerr.Fmt("unsupported aggregation method (%s).", agg)
	}

	var values []float64
	for _, digest := range digests {
		res, err := c.provider.Fetch(ctx, digest)
		if err != nil {
			return nil, skerr.Wrapf(err, "could not fetch results from CAS (%v)", digest)
		}

		// res should be map[string]*PerfResults, right now we work around with the pointer.
		pr := res[benchmark]
		samples := (&pr).GetSampleValues(chart)
		if aggMethod != nil && samples != nil {
			values = append(values, aggMethod(perfresults.Histogram{SampleValues: samples}))
		} else {
			values = append(values, samples...)
		}
	}
	return values, nil
}

func (c *perfCASClient) ReadValuesForAllCharts(ctx context.Context, benchmark string, digests []*apipb.CASReference, agg string) (map[string][]float64, error) {
	aggMethod, ok := perfresults.AggregationMapping[agg]
	if !ok && agg != "" {
		return nil, skerr.Fmt("unsupported aggregation method (%s).", agg)
	}

	valuesByChart := map[string][]float64{}
	// a digest is a CAS output from one swarming task
	for _, digest := range digests {
		res, err := c.provider.Fetch(ctx, digest)
		if err != nil {
			return nil, skerr.Wrapf(err, "could not fetch results from CAS (%v)", digest)
		}
		pr := res[benchmark]
		for k, sv := range pr.Histograms {
			var values []float64
			if aggMethod != nil && sv.SampleValues != nil {
				values = []float64{aggMethod(perfresults.Histogram{SampleValues: sv.SampleValues})}
			} else {
				values = sv.SampleValues
			}
			valuesByChart[k.ChartName] = append(valuesByChart[k.ChartName], values...)
		}
	}
	return valuesByChart, nil
}
