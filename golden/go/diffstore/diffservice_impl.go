package diffstore

import (
	context "golang.org/x/net/context"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
)

// Generate the go code from the protocol buffer definitions.
//go:generate protoc --go_out=plugins=grpc:. diffservice.proto

const MAX_MESSAGE_SIZE = 100 * 1024 * 1024

// DiffServiceImpl implements DiffServiceServer.
type DiffServiceImpl struct {
	diffStore diff.DiffStore
	codec     util.LRUCodec
}

// NewDiffServiceServer implements the server side of the diff service by
// wrapping around a DiffStore, most likely an instance of MemDiffStore.
func NewDiffServiceServer(diffStore diff.DiffStore, codec util.LRUCodec) DiffServiceServer {
	return &DiffServiceImpl{
		diffStore: diffStore,
		// The codec processes instances of map[string]interface{}. The values of
		// the map have the same underlying type as the return values of the diff
		// function that was used to instantiate the diffStore.
		codec: codec,
	}
}

// GetDiffs wraps around the Get method of the underlying DiffStore.
func (d *DiffServiceImpl) GetDiffs(ctx context.Context, req *GetDiffsRequest) (*GetDiffsResponse, error) {
	diffs, err := d.diffStore.Get(req.Priority, req.MainDigest, req.RightDigests)
	if err != nil {
		return nil, err
	}

	bytes, err := d.codec.Encode(diffs)
	if err != nil {
		return nil, err
	}

	return &GetDiffsResponse{
		Diffs: bytes,
	}, nil
}

// WarmDigests wraps around the WarmDigests method of the underlying DiffStore.
func (d *DiffServiceImpl) WarmDigests(ctx context.Context, req *WarmDigestsRequest) (*Empty, error) {
	d.diffStore.WarmDigests(req.Priority, req.Digests, req.Sync)
	return &Empty{}, nil
}

// WarmDiffs wraps around the WarmDiffs method of the underlying DiffStore.
func (d *DiffServiceImpl) WarmDiffs(ctx context.Context, req *WarmDiffsRequest) (*Empty, error) {
	d.diffStore.WarmDiffs(req.Priority, req.LeftDigests, req.RightDigests)
	return &Empty{}, nil
}

// UnavailableDigests wraps around the UnavailableDigests method of the underlying DiffStore.
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

// PurgeDigests wraps around the PurgeDigests method of the underlying DiffStore.
func (d *DiffServiceImpl) PurgeDigests(ctx context.Context, req *PurgeDigestsRequest) (*Empty, error) {
	return &Empty{}, d.diffStore.PurgeDigests(req.Digests, req.PurgeGCS)
}

// Ping returns an empty message, used to test the connection.
func (d *DiffServiceImpl) Ping(context.Context, *Empty) (*Empty, error) {
	return &Empty{}, nil
}
