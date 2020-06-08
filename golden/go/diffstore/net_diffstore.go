package diffstore

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore/common"
	"go.skia.org/infra/golden/go/types"
	"google.golang.org/grpc"
)

// NetDiffStore implements the DiffStore interface and wraps around
// a DiffService client.
type NetDiffStore struct {
	// serviceClient is the gRPC client for the DiffService.
	serviceClient DiffServiceClient

	// diffServerImageAddress is the port where the diff server serves images.
	diffServerImageAddress string

	// codec is used to decode the byte array received from the diff server into
	// a diff metrics map
	codec util.Codec
}

// NewNetDiffStore implements the diff.DiffStore interface via the gRPC-based DiffService.
func NewNetDiffStore(conn *grpc.ClientConn, diffServerImageAddress string) (diff.DiffStore, error) {
	serviceClient := NewDiffServiceClient(conn)
	if _, err := serviceClient.Ping(context.TODO(), &Empty{}); err != nil {
		return nil, skerr.Wrapf(err, "pinging connection to %s", diffServerImageAddress)
	}

	return &NetDiffStore{
		serviceClient:          serviceClient,
		diffServerImageAddress: diffServerImageAddress,
		codec:                  util.NewJSONCodec(map[types.Digest]*diff.DiffMetrics{}),
	}, nil
}

// NewForTesting returns an instantiated NetDiffStore. It should only be used by tests which are
// mocking DiffServiceClient
func NewForTesting(msc DiffServiceClient, diffServerImageAddress string) diff.DiffStore {
	return &NetDiffStore{
		serviceClient:          msc,
		diffServerImageAddress: diffServerImageAddress,
		codec:                  util.NewJSONCodec(map[types.Digest]*diff.DiffMetrics{}),
	}
}

// Get implements the diff.DiffStore interface.
func (n *NetDiffStore) Get(ctx context.Context, mainDigest types.Digest, rightDigests types.DigestSlice) (map[types.Digest]*diff.DiffMetrics, error) {
	req := &GetDiffsRequest{MainDigest: string(mainDigest), RightDigests: common.AsStrings(rightDigests)}
	resp, err := n.serviceClient.GetDiffs(ctx, req)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	data, err := n.codec.Decode(resp.Diffs)
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not decode response")
	}

	diffMetrics, ok := data.(map[types.Digest]*diff.DiffMetrics)
	if !ok {
		return nil, skerr.Fmt("Not a valid diff metrics: %#v", data)
	}
	return diffMetrics, nil
}

// ImageHandler implements the diff.DiffStore interface. The images are expected to be served
// by the server that implements the backend of the DiffService.
func (n *NetDiffStore) ImageHandler(urlPrefix string) (http.Handler, error) {
	// Set up a proxy to the diffserver images ports.
	// With ingress rules, it would be possible to serve directly to the diffserver
	// and bypass the main server.
	targetURL, err := url.Parse(fmt.Sprintf("http://%s", n.diffServerImageAddress))
	if err != nil {
		return nil, skerr.Wrapf(err, "invalid URL for serving diff images")
	}
	return httputil.NewSingleHostReverseProxy(targetURL), nil
}

// UnavailableDigests implements the diff.DiffStore interface.
func (n *NetDiffStore) UnavailableDigests(ctx context.Context) (map[types.Digest]*diff.DigestFailure, error) {
	resp, err := n.serviceClient.UnavailableDigests(ctx, &Empty{})
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	ret := make(map[types.Digest]*diff.DigestFailure, len(resp.DigestFailures))
	for k, failure := range resp.DigestFailures {
		ret[types.Digest(k)] = &diff.DigestFailure{
			Digest: types.Digest(failure.Digest),
			Reason: diff.DiffErr(failure.Reason),
			TS:     failure.TS,
		}
	}
	return ret, nil
}

// PurgeDigests implements the the diff.DiffStore interface.
func (n *NetDiffStore) PurgeDigests(ctx context.Context, digests types.DigestSlice, purgeGCS bool) error {
	req := &PurgeDigestsRequest{Digests: common.AsStrings(digests), PurgeGCS: purgeGCS}
	_, err := n.serviceClient.PurgeDigests(ctx, req)
	if err != nil {
		return skerr.Wrapf(err, "purging %d digests %s", len(digests), digests)
	}
	return nil
}
