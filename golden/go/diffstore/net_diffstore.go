package diffstore

import (
	"fmt"
	"net/http"

	context "golang.org/x/net/context"
	"google.golang.org/grpc"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/diff"
)

// NetDiffStore implements the DiffStore interface and wraps around
// a DiffService client.
type NetDiffStore struct {
	serviceClient DiffServiceClient
}

// NewNetDiffStore implements the diff.DiffStore interface via the gRPC-based DiffService.
func NewNetDiffStore(conn *grpc.ClientConn) (diff.DiffStore, error) {
	serviceClient := NewDiffServiceClient(conn)
	if _, err := serviceClient.Ping(context.Background(), &Empty{}); err != nil {
		return nil, fmt.Errorf("Could not ping over connection: %s", err)
	}

	return &NetDiffStore{
		serviceClient: serviceClient,
	}, nil
}

// Get, see the diff.DiffStore interface.
func (n *NetDiffStore) Get(priority int64, mainDigest string, rightDigests []string) (map[string]*diff.DiffMetrics, error) {
	req := &GetDiffsRequest{Priority: priority, MainDigest: mainDigest, RightDigests: rightDigests}
	resp, err := n.serviceClient.GetDiffs(context.Background(), req)
	if err != nil {
		return nil, err
	}
	ret := make(map[string]*diff.DiffMetrics, len(resp.Diffs))
	for k, metrics := range resp.Diffs {
		ret[k] = toDiffMetrics(metrics)
	}
	return ret, nil
}

// ImageHandler, see the diff.DiffStore interface. This is not implemented and
// will always return an error. The images are expected to be served by the
// the server that implements the backend of the DiffService.
func (n *NetDiffStore) ImageHandler(urlPrefix string) (http.Handler, error) {
	return nil, fmt.Errorf("Not implemented.")
}

// WarmDigests, see the diff.DiffStore interface.
func (n *NetDiffStore) WarmDigests(priority int64, digests []string, sync bool) {
	req := &WarmDigestsRequest{Priority: priority, Digests: digests, Sync: sync}
	_, err := n.serviceClient.WarmDigests(context.Background(), req)
	if err != nil {
		sklog.Errorf("Error warming digests: %s", err)
	}
}

// WarmDiffs, see the diff.DiffStore interface.
func (n *NetDiffStore) WarmDiffs(priority int64, leftDigests []string, rightDigests []string) {
	req := &WarmDiffsRequest{Priority: priority, LeftDigests: leftDigests, RightDigests: rightDigests}
	_, err := n.serviceClient.WarmDiffs(context.Background(), req)
	if err != nil {
		sklog.Errorf("Error warming diffs: %s", err)
	}
}

// UnavailableDigests, see the diff.DiffStore interface.
func (n *NetDiffStore) UnavailableDigests() map[string]*diff.DigestFailure {
	resp, err := n.serviceClient.UnavailableDigests(context.Background(), &Empty{})
	if err != nil {
		return map[string]*diff.DigestFailure{}
	}

	ret := make(map[string]*diff.DigestFailure, len(resp.DigestFailures))
	for k, failure := range resp.DigestFailures {
		ret[k] = &diff.DigestFailure{
			Digest: failure.Digest,
			Reason: diff.DiffErr(failure.Reason),
			TS:     failure.TS,
		}
	}
	return ret
}

// PurgeDigests, see the diff.DiffStore interface.
func (n *NetDiffStore) PurgeDigests(digests []string, purgeGCS bool) error {
	req := &PurgeDigestsRequest{Digests: digests, PurgeGCS: purgeGCS}
	_, err := n.serviceClient.PurgeDigests(context.Background(), req)
	if err != nil {
		return fmt.Errorf("Error purging digests: %s", err)
	}
	return nil
}

// toDifMetrics converts a DiffMetrics response to the equivalent instance
// of diff.DiffMetrics.
func toDiffMetrics(d *DiffMetricsResponse) *diff.DiffMetrics {
	return &diff.DiffMetrics{
		NumDiffPixels:    int(d.NumDiffPixels),
		PixelDiffPercent: d.PixelDiffPercent,
		MaxRGBADiffs:     toIntSlice(d.MaxRGBADiffs),
		DimDiffer:        d.DimDiffer,
		Diffs:            d.Diffs,
	}
}

func toIntSlice(arr []int32) []int {
	ret := make([]int, len(arr))
	for idx, val := range arr {
		ret[idx] = int(val)
	}
	return ret
}
