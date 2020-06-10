package diffstore

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/types"
)

// Generate the go code from the protocol buffer definitions.
//go:generate protoc --go_out=plugins=grpc:. diffservice.proto
//go:generate goimports -w diffservice.pb.go

const MAX_MESSAGE_SIZE = 100 * 1024 * 1024

// DiffServiceImpl implements DiffServiceServer.
type DiffServiceImpl struct {
	diffStore diff.DiffStore
	codec     util.Codec
}

// NewDiffServiceServer implements the server side of the diff service by
// wrapping around a DiffStore, most likely an instance of MemDiffStore.
func NewDiffServiceServer(diffStore diff.DiffStore) DiffServiceServer {
	return &DiffServiceImpl{
		diffStore: diffStore,
		codec:     util.NewJSONCodec(map[types.Digest]*diff.DiffMetrics{}),
	}
}

// TODO(kjlubick): Is there a way to tell the protobuf to use the types.Digest directly?
func asDigests(xs []string) types.DigestSlice {
	d := make(types.DigestSlice, 0, len(xs))
	for _, s := range xs {
		d = append(d, types.Digest(s))
	}
	return d
}

// GetDiffs wraps around the Get method of the underlying DiffStore.
func (d *DiffServiceImpl) GetDiffs(ctx context.Context, req *GetDiffsRequest) (*GetDiffsResponse, error) {
	diffs, err := d.diffStore.Get(ctx, types.Digest(req.MainDigest), asDigests(req.RightDigests))
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	bytes, err := d.codec.Encode(diffs)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &GetDiffsResponse{
		Diffs: bytes,
	}, nil
}

// UnavailableDigests wraps around the UnavailableDigests method of the underlying DiffStore.
func (d *DiffServiceImpl) UnavailableDigests(ctx context.Context, req *Empty) (*UnavailableDigestsResponse, error) {
	return &UnavailableDigestsResponse{DigestFailures: map[string]*DigestFailureResponse{}}, nil
}

// PurgeDigests wraps around the PurgeDigests method of the underlying DiffStore.
func (d *DiffServiceImpl) PurgeDigests(ctx context.Context, req *PurgeDigestsRequest) (*Empty, error) {
	return &Empty{}, d.diffStore.PurgeDigests(ctx, asDigests(req.Digests), req.PurgeGCS)
}

// Ping returns an empty message, used to test the connection.
func (d *DiffServiceImpl) Ping(context.Context, *Empty) (*Empty, error) {
	return &Empty{}, nil
}
