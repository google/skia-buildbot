package diffstore

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	lru "github.com/hashicorp/golang-lru"
	ttlcache "github.com/patrickmn/go-cache"
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

	// Caches diffs (which are immutable) to lessen the network load
	diffsCache *lru.Cache

	unavailableCache *ttlcache.Cache
}

const (
	// A DiffMetrics object is about 60 bytes, so holding up to 10 million shouldn't
	// put too much pressure on our memory usage.
	diffCacheSize = 10 * 1000 * 1000

	unavailableFreshness    = 30 * time.Second
	unavailableCacheCleanup = 2 * time.Minute
	unavailableKey          = "unavailable"
)

type cacheKey struct {
	Left  types.Digest
	Right types.Digest
}

// NewNetDiffStore implements the diff.DiffStore interface via the gRPC-based DiffService.
func NewNetDiffStore(conn *grpc.ClientConn, diffServerImageAddress string) (diff.DiffStore, error) {
	serviceClient := NewDiffServiceClient(conn)
	if _, err := serviceClient.Ping(context.Background(), &Empty{}); err != nil {
		return nil, skerr.Wrapf(err, "pinging connection to %s", diffServerImageAddress)
	}
	c, err := lru.New(diffCacheSize)
	if err != nil {
		return nil, skerr.Wrapf(err, "making cache of size %d", diffCacheSize)
	}

	return &NetDiffStore{
		serviceClient:          serviceClient,
		diffServerImageAddress: diffServerImageAddress,
		codec:                  util.NewJSONCodec(map[types.Digest]*diff.DiffMetrics{}),
		diffsCache:             c,
		unavailableCache:       ttlcache.New(unavailableFreshness, unavailableCacheCleanup),
	}, nil
}

// NewForTesting returns an instantiated NetDiffStore. It should only be used by tests which are
// mocking DiffServiceClient
func NewForTesting(msc DiffServiceClient, diffServerImageAddress string) (diff.DiffStore, error) {
	// Don't need quite as big an LRU for testing
	const testCacheSize = 1000
	c, err := lru.New(testCacheSize)
	if err != nil {
		return nil, skerr.Wrapf(err, "making cache of size %d", testCacheSize)
	}

	return &NetDiffStore{
		serviceClient:          msc,
		diffServerImageAddress: diffServerImageAddress,
		codec:                  util.NewJSONCodec(map[types.Digest]*diff.DiffMetrics{}),
		diffsCache:             c,
		unavailableCache:       ttlcache.New(unavailableFreshness, unavailableCacheCleanup),
	}, nil
}

// Get implements the diff.DiffStore interface.
func (n *NetDiffStore) Get(ctx context.Context, mainDigest types.Digest, rightDigests types.DigestSlice) (map[types.Digest]*diff.DiffMetrics, error) {
	ret := make(map[types.Digest]*diff.DiffMetrics, len(rightDigests))
	var cacheMisses types.DigestSlice
	for _, d := range rightDigests {
		ck := cacheKey{
			Left:  mainDigest,
			Right: d,
		}
		if cached, ok := n.diffsCache.Get(ck); ok {
			dm, ok := cached.(*diff.DiffMetrics)
			if !ok {
				// cache was corrupted somehow. Remove the bad entry and re-load it.
				n.diffsCache.Remove(ck)
				cacheMisses = append(cacheMisses, d)
				continue
			}
			ret[d] = dm
		} else {
			cacheMisses = append(cacheMisses, d)
		}
	}

	if len(cacheMisses) > 0 {
		req := &GetDiffsRequest{MainDigest: string(mainDigest), RightDigests: common.AsStrings(cacheMisses)}
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
		for d, dm := range diffMetrics {
			ck := cacheKey{
				Left:  mainDigest,
				Right: d,
			}
			n.diffsCache.Add(ck, dm)
			ret[d] = dm
		}
	}

	return ret, nil
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
	if cached, ok := n.unavailableCache.Get(unavailableKey); ok {
		if df, ok := cached.(map[types.Digest]*diff.DigestFailure); ok {
			return df, nil
		}
		// bad entry in cache, clear it and keep going.
		n.unavailableCache.Delete(unavailableKey)
	}

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
	n.unavailableCache.SetDefault(unavailableKey, ret)
	return ret, nil
}

// PurgeDigests implements the the diff.DiffStore interface.
func (n *NetDiffStore) PurgeDigests(ctx context.Context, digests types.DigestSlice, purgeGCS bool) error {
	// It is easiest to purge all caches if we need to purge anything, just to be sure
	// they are cleared.
	n.diffsCache.Purge()
	n.unavailableCache.Delete(unavailableKey)

	req := &PurgeDigestsRequest{Digests: common.AsStrings(digests), PurgeGCS: purgeGCS}
	_, err := n.serviceClient.PurgeDigests(ctx, req)
	if err != nil {
		return skerr.Wrapf(err, "purging %d digests %s", len(digests), digests)
	}
	return nil
}
