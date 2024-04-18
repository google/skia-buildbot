package perfresults

import (
	"bytes"
	"context"
	"strings"

	"github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/digest"
	repb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	swarmingv2 "go.chromium.org/luci/swarming/proto/api_v2"

	"go.skia.org/infra/go/skerr"
)

const (
	rbeServiceAddress = "remotebuildexecution.googleapis.com:443"
	perfJsonFilename  = "perf_results.json"
)

// RBEPerfLoader wraps rbe.Client to provide convenient functions
type RBEPerfLoader struct {
	*client.Client
}

func NewRBEPerfLoader(ctx context.Context, casInstance string) (*RBEPerfLoader, error) {
	c, err := client.NewClient(ctx, casInstance, client.DialParams{
		Service:               rbeServiceAddress,
		UseApplicationDefault: true,
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "unable to create new RBE client")
	}
	return &RBEPerfLoader{Client: c}, nil
}

// fetchPerfDigests returns the digests from the given CAS output of a swarming task
func (c RBEPerfLoader) fetchPerfDigests(ctx context.Context, cas *swarmingv2.CASReference) (map[string]digest.Digest, error) {
	if c.InstanceName != cas.GetCasInstance() {
		return nil, skerr.Fmt("cas ref is from a different instance (%s vs %s)", c.InstanceName, cas.GetCasInstance())
	}

	d, err := digest.New(cas.Digest.Hash, cas.Digest.SizeBytes)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	var rootDir repb.Directory
	if _, err := c.ReadProto(ctx, d, &rootDir); err != nil {
		return nil, skerr.Wrap(err)
	}

	dirs, err := c.GetDirectoryTree(ctx, d.ToProto())
	if err != nil {
		return nil, skerr.Wrapf(err, "unable to get dir tree for CAS (%s)", cas.String())
	}

	outputs, err := c.FlattenTree(&repb.Tree{
		Root:     &rootDir,
		Children: dirs,
	}, "")
	if err != nil {
		return nil, skerr.Wrapf(err, "unable to flatten tree for CAS (%s)", cas.String())
	}

	perfDigests := make(map[string]digest.Digest)
	for path, output := range outputs {
		if !strings.HasSuffix(path, perfJsonFilename) {
			continue
		}
		parts := strings.Split(path, "/")
		if len(parts) != 2 {
			return nil, skerr.Fmt("perf file location (%s) is unexpected", path)
		}
		perfDigests[parts[0]] = output.Digest
	}
	return perfDigests, nil
}

// loadPerfResult expects the JSON content from the given digest and loads into PerfResults.
func (c RBEPerfLoader) loadPerfResult(ctx context.Context, digest digest.Digest) (*PerfResults, error) {
	blob, _, err := c.ReadBlob(ctx, digest)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return NewResults(bytes.NewBuffer(blob))
}

// LoadPerfResults loads all the perf_results.json from the list of CAS outputs of swarming tasks.
//
// The CAS output should point to the root folder of the swarming task.
func (c RBEPerfLoader) LoadPerfResults(ctx context.Context, cases ...*swarmingv2.CASReference) (map[string]*PerfResults, error) {
	results := make(map[string]*PerfResults)

	// This can be done in parallel, but the gain seems to be minimum and it increases complexity
	// by lot. We keep it simple here unless we run into performance issues.
	for _, cas := range cases {
		digests, err := c.fetchPerfDigests(ctx, cas)
		if err != nil {
			return nil, skerr.Wrap(err)
		}

		for benchmark, digest := range digests {
			pr, err := c.loadPerfResult(ctx, digest)
			if err != nil {
				return nil, skerr.Wrap(err)
			}

			if e, ok := results[benchmark]; ok {
				e.MergeResults(pr)
			} else {
				results[benchmark] = pr
			}
		}
	}

	return results, nil
}
