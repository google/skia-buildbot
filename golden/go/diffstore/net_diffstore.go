package diffstore

import (
	"context"
	"net/http"
	"net/http/httputil"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
)

// NetDiffStore implements the DiffStore interface and wraps around
// a DiffService client.
type NetDiffStore struct {
	// serviceClient is the gRPC client for the DiffService.
	// serviceClient DiffServiceClient
	backendMap map[string]*backendService

	// diffServerImageAddress is the port where the diff server serves images.
	diffServerImageAddress string

	// codec is used to decode the byte array received from the diff server into
	// a diff metrics map
	codec util.LRUCodec
}

type backendService struct {
	serviceClient DiffServiceClient
	imgProxy      *httputil.ReverseProxy
	imageAddress  string

	// targetURL, err := url.Parse(fmt.Sprintf("http://%s", n.diffServerImageAddress))
	// if err != nil {
	// 	return nil, fmt.Errorf("Invalid address for serving diff images. Got error: %s", err)
	// }
	// return httputil.NewSingleHostReverseProxy(targetURL), nil

}

// NewNetDiffStore implements the diff.DiffStore interface via the gRPC-based DiffService.
func NewNetDiffStore(diffserverAddrs []string, basePort int, codec util.LRUCodec) (diff.DiffStore, error) {
	// Set up the backends
	// conn *grpc.ClientConn,
	// for _, addr := range diffserverAddrs {
	// 	grpcAddr := fmt.Sprintf("%s:%d", addr, basePort)
	// 	imgAddr := fmt.Sprintf("%s:%d", addr, basePort+1)

	// }

	// serviceClient := NewDiffServiceClient(conn)
	// if _, err := serviceClient.Ping(context.Background(), &Empty{}); err != nil {
	// 	return nil, fmt.Errorf("Could not ping over connection: %s", err)
	// }

	return &NetDiffStore{
		// serviceClient:          serviceClient,
		// diffServerImageAddress: diffServerImageAddress,
		codec: codec,
	}, nil
}

func (n *NetDiffStore) getShardedBackend(shardKey string) *backendService {
	return nil
}

// Get, see the diff.DiffStore interface.
func (n *NetDiffStore) Get(priority int64, shardKey string, mainDigest string, rightDigests []string) (map[string]interface{}, error) {
	req := &GetDiffsRequest{Priority: priority, MainDigest: mainDigest, RightDigests: rightDigests}
	backend := n.getShardedBackend(shardKey)
	resp, err := backend.serviceClient.GetDiffs(context.Background(), req)
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

// ImageHandler, see the diff.DiffStore interface.
func (n *NetDiffStore) ImageHandler(urlPrefix string) (http.Handler, error) {
	// Set up a proxy to the differ server images ports. In production
	// this should not be really used since we proxy directly from the frontend
	// to the diff server.

	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		// Process the path and extract images.
		shardKey, _, _, valid := ParseImagePath(r.URL.Path)
		if !valid {
			noCacheNotFound(w, r)
			return
		}

		backend := n.getShardedBackend(shardKey)
		backend.imgProxy.ServeHTTP(w, r)
	}
	return http.HandlerFunc(handlerFunc), nil
}

// WarmDigests, see the diff.DiffStore interface.
func (n *NetDiffStore) WarmDigests(priority int64, shardKey string, digests []string, sync bool) {
	backend := n.getShardedBackend(shardKey)
	req := &WarmDigestsRequest{Priority: priority, Digests: digests, Sync: sync}
	_, err := backend.serviceClient.WarmDigests(context.Background(), req)
	if err != nil {
		sklog.Errorf("Error warming digests: %s", err)
	}
}

// WarmDiffs, see the diff.DiffStore interface.
func (n *NetDiffStore) WarmDiffs(priority int64, shardKey string, leftDigests []string, rightDigests []string) {
	backend := n.getShardedBackend(shardKey)
	req := &WarmDiffsRequest{Priority: priority, LeftDigests: leftDigests, RightDigests: rightDigests}
	_, err := backend.serviceClient.WarmDiffs(context.Background(), req)
	if err != nil {
		sklog.Errorf("Error warming diffs: %s", err)
	}
}

// UnavailableDigests, see the diff.DiffStore interface.
func (n *NetDiffStore) UnavailableDigests() map[string]*diff.DigestFailure {
	ret := map[string]*diff.DigestFailure{}
	for _, backend := range n.backendMap {
		resp, err := backend.serviceClient.UnavailableDigests(context.Background(), &Empty{})
		if err != nil {
			return map[string]*diff.DigestFailure{}
		}

		for k, failure := range resp.DigestFailures {
			ret[k] = &diff.DigestFailure{
				Digest: failure.Digest,
				Reason: diff.DiffErr(failure.Reason),
				TS:     failure.TS,
			}
		}
	}
	return ret
}

// PurgeDigests, see the diff.DiffStore interface.
func (n *NetDiffStore) PurgeDigests(digests []string, purgeGCS bool) error {
	for _, backend := range n.backendMap {
		req := &PurgeDigestsRequest{Digests: digests, PurgeGCS: purgeGCS}
		_, err := backend.serviceClient.PurgeDigests(context.Background(), req)
		if err != nil {
			sklog.Errorf("Error purging digests: %s", err)
			continue
		}
	}
	return nil
}
