package diffstore

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"google.golang.org/grpc"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
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
	codec util.LRUCodec
}

// NewNetDiffStore implements the diff.DiffStore interface via the gRPC-based DiffService.
func NewNetDiffStore(conn *grpc.ClientConn, diffServerImageAddress string, codec util.LRUCodec) (diff.DiffStore, error) {
	serviceClient := NewDiffServiceClient(conn)
	if _, err := serviceClient.Ping(context.Background(), &Empty{}); err != nil {
		return nil, fmt.Errorf("Could not ping over connection: %s", err)
	}

	return &NetDiffStore{
		serviceClient:          serviceClient,
		diffServerImageAddress: diffServerImageAddress,
		codec:                  codec,
	}, nil
}

// Get, see the diff.DiffStore interface.
func (n *NetDiffStore) Get(priority int64, mainDigest string, rightDigests []string) (map[string]interface{}, error) {
	req := &GetDiffsRequest{Priority: priority, MainDigest: mainDigest, RightDigests: rightDigests}
	resp, err := n.serviceClient.GetDiffs(context.Background(), req)
	if err != nil {
		return nil, err
	}

	data, err := n.codec.Decode(resp.Diffs)
	if err != nil {
		return nil, err
	}

	diffMetrics := data.(map[string]interface{})
	return diffMetrics, nil
}

// ImageHandler, see the diff.DiffStore interface. This is not implemented and
// will always return an error. The images are expected to be served by the
// the server that implements the backend of the DiffService.
func (n *NetDiffStore) ImageHandler(urlPrefix string) (http.Handler, error) {
	// Set up a proxy to the differ server images ports. In production
	// this should not be really used since we proxy directly from the frontend
	// to the diff server.

	targetURL, err := url.Parse(fmt.Sprintf("http://%s", n.diffServerImageAddress))
	if err != nil {
		return nil, fmt.Errorf("Invalid address for serving diff images. Got error: %s", err)
	}
	return httputil.NewSingleHostReverseProxy(targetURL), nil
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
