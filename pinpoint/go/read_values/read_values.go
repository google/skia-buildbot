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

var aggregationMapping = map[string]func(perfresults.Histogram) float64{
	"max":  perfresults.Histogram.Max,
	"min":  perfresults.Histogram.Min,
	"mean": perfresults.Histogram.Mean,
	"std":  perfresults.Histogram.Stddev,
	"sum":  perfresults.Histogram.Sum,
	"count": func(h perfresults.Histogram) float64 {
		return float64(h.Count())
	},
}

func IsSupportedAggregation(aggregationMethod string) bool {
	if aggregationMethod == "" {
		return true
	}
	if _, ok := aggregationMapping[aggregationMethod]; ok {
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

// ReadValuesByChart reads Pinpoint results for specific benchmark and chart from a list of CAS digests.
// ReadValuesByChart will also apply any data aggregations.
//
// Example Usage:
//
//	ctx := context.Background()
//	client, err := DialRBECAS(ctx)
//	values := client.ReadValuesByChart(ctx, benchmark, chart, digests, nil)
//
// TODO(sunxiaodi@): Migrate CABE backends into pinpoint/go/backends/
func (c *perfCASClient) ReadValuesByChart(ctx context.Context, benchmark string, chart string, digests []*apipb.CASReference, agg string) ([]float64, error) {
	aggMethod, ok := aggregationMapping[agg]
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
