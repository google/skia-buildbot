package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.skia.org/infra/cabe/go/perfresults"
	"go.skia.org/infra/go/sklog"

	rbeclient "github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/digest"
	repb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
)

const (
	resultsFilename = "perf_results.json"
)

// FetchBenchmarkJSON fetches the benchmark results json files from the CAS root at rootDigest.
// It returns a map of benchmark name to perfresults.PerfResults parsed from the json bytes.
func FetchBenchmarkJSON(ctx context.Context, c *rbeclient.Client, rootDigest string) (map[string]perfresults.PerfResults, error) {
	raw, err := FetchBenchmarkJSONRaw(ctx, c, rootDigest)
	if err != nil {
		return nil, err
	}
	ret := make(map[string]perfresults.PerfResults)
	for benchmark, blob := range raw {
		res := perfresults.PerfResults{}
		if err := json.Unmarshal(blob, &res); err != nil {
			sklog.Errorf("unmarshaling benchmark json: %v", err)
			return nil, err
		}
		ret[benchmark] = res
	}
	return ret, nil
}

// FetchBenchmarkJSONRaw fetches the benchmark results json files from the CAS root at rootDigest.
// It returns a map of benchmark name to raw json bytes.
func FetchBenchmarkJSONRaw(ctx context.Context, c *rbeclient.Client, rootDigest string) (map[string][]byte, error) {
	d, err := digest.NewFromString(rootDigest)
	if err != nil {
		return nil, fmt.Errorf("failed to parse digest %q: %v", rootDigest, err)
	}
	rootDir := &repb.Directory{}
	if err := c.ReadProto(ctx, d, rootDir); err != nil {
		return nil, fmt.Errorf("failed to read root directory proto: %v", err)
	}

	dirs, err := c.GetDirectoryTree(ctx, d.ToProto())
	if err != nil {
		return nil, fmt.Errorf("failed to call GetDirectoryTree: %v", err)
	}

	t := &repb.Tree{
		Root:     rootDir,
		Children: dirs,
	}

	outputs, err := c.FlattenTree(t, "")
	if err != nil {
		return nil, fmt.Errorf("failed to call FlattenTree: %v", err)
	}

	ret := make(map[string][]byte)

	for path, output := range outputs {
		if strings.HasSuffix(path, resultsFilename) {
			parts := strings.Split(path, "/")
			if len(parts) != 2 {
				sklog.Errorf("unexpected output path format: %q", path)
			}
			benchmark := parts[0]
			d, err := digest.New(output.Digest.Hash, output.Digest.Size)
			if err != nil {
				return nil, err
			}
			blob, err := c.ReadBlob(ctx, d)
			if err != nil {
				return nil, err
			}

			ret[benchmark] = blob
		}
	}

	return ret, nil
}

// ParseBenchmarkJSON parse the content of perf_results.json
// It returns a map of benchmark name to perfresults.PerfResults parsed from the json bytes.
func ParseBenchmarkJSON(benchmark string, perfResultJSON string) (map[string]perfresults.PerfResults, error) {
	ret := make(map[string]perfresults.PerfResults)

	res := perfresults.PerfResults{}
	if err := json.Unmarshal([]byte(perfResultJSON), &res); err != nil {
		sklog.Errorf("unmarshaling benchmark json: %v", err)
		return nil, err
	}
	ret[benchmark] = res

	return ret, nil
}
