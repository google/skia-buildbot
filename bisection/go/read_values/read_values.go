package read_values

import (
	"context"
	"fmt"

	"go.skia.org/infra/cabe/go/backends"
	"go.skia.org/infra/cabe/go/perfresults"
	"go.skia.org/infra/go/sklog"

	rbeclient "github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
)

// Dial RBE CAS client given a swarming instance.
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

// Reads Pinpoint results for specific benchmark and chart from a list of CAS digests
// Example Usage:
//
//	ctx := context.Background()
//	client, err := DialRBECAS(ctx)
//	values := client.ReadValuesByChart(ctx, client, benchmark, chart, digests)
func ReadValuesByChart(ctx context.Context, client *rbeclient.Client, benchmark string, chart string, digests []string) []float64 {
	values := []float64{}
	for _, digest := range digests {
		res, _ := backends.FetchBenchmarkJSON(ctx, client, digest)
		v := ReadChart(res, benchmark, chart)
		values = append(values, v...)
	}
	return values
}

// Extracts specific benchmark and chart data from one CAS digest
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
