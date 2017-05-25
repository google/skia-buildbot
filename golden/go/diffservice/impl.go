package diffservice

import (
	"fmt"
	"net/http"

	context "golang.org/x/net/context"
	"google.golang.org/grpc"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/diff"
)

// Generate the go code from the protocol buffer definitions.
//go:generate protoc --go_out=plugins=grpc:. diffservice.proto

// DiffServiceImpl implements DiffServiceServer.
type DiffServiceImpl struct {
	diffStore diff.DiffStore
}

func NewDiffServiceServer(diffStore diff.DiffStore) DiffServiceServer {
	return &DiffServiceImpl{
		diffStore: diffStore,
	}
}

func (d *DiffServiceImpl) GetDiffs(ctx context.Context, req *GetDiffsRequest) (*GetDiffsResponse, error) {
	diffs, err := d.diffStore.Get(req.Priority, req.MainDigest, req.RightDigests)
	if err != nil {
		return nil, err
	}

	resp := make(map[string]*DiffMetricsResponse, len(diffs))
	for k, metrics := range diffs {
		resp[k] = toDiffMetricsResponse(metrics)
	}

	return &GetDiffsResponse{
		Diffs: resp,
	}, nil
}

func toDiffMetricsResponse(d *diff.DiffMetrics) *DiffMetricsResponse {
	return &DiffMetricsResponse{
		NumDiffPixels:    int32(d.NumDiffPixels),
		PixelDiffPercent: d.PixelDiffPercent,
		MaxRGBADiffs:     toInt32Slice(d.MaxRGBADiffs),
		DimDiffer:        d.DimDiffer,
		Diffs:            d.Diffs,
	}
}

func toInt32Slice(arr []int) []int32 {
	ret := make([]int32, len(arr))
	for idx, val := range arr {
		ret[idx] = int32(val)
	}
	return ret
}

func (d *DiffServiceImpl) WarmDigests(ctx context.Context, req *WarmDigestsRequest) (*Empty, error) {
	d.diffStore.WarmDigests(req.Priority, req.Digests)
	return &Empty{}, nil
}

func (d *DiffServiceImpl) WarmDiffs(ctx context.Context, req *WarmDiffsRequest) (*Empty, error) {
	d.diffStore.WarmDiffs(req.Priority, req.LeftDigests, req.RightDigests)
	return &Empty{}, nil
}

func (d *DiffServiceImpl) UnavailableDigests(ctx context.Context, req *Empty) (*UnavailableDigestsResponse, error) {
	unavailable := d.diffStore.UnavailableDigests()
	ret := make(map[string]*DigestFailureResponse, len(unavailable))
	for k, failure := range unavailable {
		ret[k] = &DigestFailureResponse{
			Digest: failure.Digest,
			Reason: string(failure.Reason),
			TS:     failure.TS,
		}
	}
	return &UnavailableDigestsResponse{DigestFailures: ret}, nil
}

func (d *DiffServiceImpl) PurgeDigests(ctx context.Context, req *PurgeDigestsRequest) (*Empty, error) {
	return &Empty{}, d.diffStore.PurgeDigests(req.Digests, req.PurgeGCS)
}

func (d *DiffServiceImpl) Ping(context.Context, *Empty) (*Empty, error) {
	return &Empty{}, nil
}

// NetDiffStore implements the DiffStore interface.
type NetDiffStore struct {
	serviceClient DiffServiceClient
}

func NewNetDiffStore(conn *grpc.ClientConn) (diff.DiffStore, error) {
	serviceClient := NewDiffServiceClient(conn)
	if _, err := serviceClient.Ping(context.Background(), &Empty{}); err != nil {
		return nil, err
	}

	return &NetDiffStore{
		serviceClient: serviceClient,
	}, nil
}

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

func (n *NetDiffStore) ImageHandler(urlPrefix string) (http.Handler, error) {
	return nil, fmt.Errorf("Not implemented.")
}

func (n *NetDiffStore) WarmDigests(priority int64, digests []string) {
	req := &WarmDigestsRequest{Priority: priority, Digests: digests}
	_, err := n.serviceClient.WarmDigests(context.Background(), req)
	if err != nil {
		sklog.Errorf("Error warming digests: %s", err)
	}
}

func (n *NetDiffStore) WarmDiffs(priority int64, leftDigests []string, rightDigests []string) {
	req := &WarmDiffsRequest{Priority: priority, LeftDigests: leftDigests, RightDigests: rightDigests}
	_, err := n.serviceClient.WarmDiffs(context.Background(), req)
	if err != nil {
		sklog.Errorf("Error warming diffs: %s", err)
	}
}

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

func (n *NetDiffStore) PurgeDigests(digests []string, purgeGCS bool) error {
	req := &PurgeDigestsRequest{Digests: digests, PurgeGCS: purgeGCS}
	_, err := n.serviceClient.PurgeDigests(context.Background(), req)
	if err != nil {
		return fmt.Errorf("Error purging digests: %s", err)
	}
	return nil
}
