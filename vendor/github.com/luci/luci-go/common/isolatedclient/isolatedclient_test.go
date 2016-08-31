// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package isolatedclient

import (
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/api/isolate/isolateservice/v1"
	"github.com/luci/luci-go/common/isolated"
	"github.com/luci/luci-go/common/isolatedclient/isolatedfake"
	"github.com/luci/luci-go/common/retry"
	"github.com/maruel/ut"
)

func TestIsolateServerCaps(t *testing.T) {
	ctx := context.Background()

	t.Parallel()
	server := isolatedfake.New()
	ts := httptest.NewServer(server)
	defer ts.Close()
	client := New(nil, nil, ts.URL, DefaultNamespace, nil)
	caps, err := client.ServerCapabilities(ctx)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, &isolateservice.HandlersEndpointsV1ServerDetails{ServerVersion: "v1"}, caps)
	ut.AssertEqual(t, nil, server.Error())
}

func TestIsolateServerSmall(t *testing.T) {
	t.Parallel()
	testNormal(context.Background(), t, foo, bar)
}

func TestIsolateServerLarge(t *testing.T) {
	t.Parallel()
	testNormal(context.Background(), t, large)
}

func TestIsolateServerRetryContains(t *testing.T) {
	t.Parallel()
	testFlaky(context.Background(), t, "/api/isolateservice/v1/preupload")
}

func TestIsolateServerRetryStoreInline(t *testing.T) {
	t.Parallel()
	testFlaky(context.Background(), t, "/api/isolateservice/v1/store_inline")
}

func TestIsolateServerRetryGCS(t *testing.T) {
	t.Parallel()
	testFlaky(context.Background(), t, "/fake/cloudstorage")
}

func TestIsolateServerRetryFinalize(t *testing.T) {
	t.Parallel()
	testFlaky(context.Background(), t, "/api/isolateservice/v1/finalize_gs_upload")
}

func TestIsolateServerRetryGCSPartial(t *testing.T) {
	ctx := context.Background()

	// GCS upload is teared down in the middle.
	t.Parallel()
	server := isolatedfake.New()
	flaky := &killingMux{server: server, tearDown: map[string]int{"/fake/cloudstorage": 1024}}
	flaky.ts = httptest.NewServer(flaky)
	defer flaky.ts.Close()
	client := New(nil, nil, flaky.ts.URL, DefaultNamespace, fastRetry)

	digests, contents, expected := makeItems(large)
	states, err := client.Contains(ctx, digests)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, len(digests), len(states))
	for _, state := range states {
		err = client.Push(ctx, state, NewBytesSource(contents[state.status.Index]))
		ut.AssertEqual(t, nil, err)
	}
	ut.AssertEqual(t, expected, server.Contents())
	ut.AssertEqual(t, map[string]int{}, flaky.tearDown)

	// Look up again to confirm.
	states, err = client.Contains(ctx, digests)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, len(digests), len(states))
	for _, state := range states {
		ut.AssertEqual(t, (*PushState)(nil), state)
	}
	ut.AssertEqual(t, nil, server.Error())
}

func TestIsolateServerBadURL(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.SkipNow()
	}
	client := New(nil, nil, "http://127.0.0.1:1", DefaultNamespace, fastRetry)
	caps, err := client.ServerCapabilities(context.Background())
	ut.AssertEqual(t, (*isolateservice.HandlersEndpointsV1ServerDetails)(nil), caps)
	ut.AssertEqual(t, true, err != nil)
}

// Private stuff.

func cantRetry() retry.Iterator {
	return &retry.ExponentialBackoff{
		Limited: retry.Limited{
			Retries: 10,
			Delay:   10 * time.Second,
		},
		Multiplier: 10,
	}
}

func fastRetry() retry.Iterator {
	return &retry.Limited{
		Retries: 10,
	}
}

var (
	bar   = []byte("bar")
	foo   = []byte("foo")
	large []byte
)

func init() {
	src := rand.New(rand.NewSource(0))
	// It has to be exactly 64kb + 1; as the internal buffering is 64kb. This is
	// needed to reproduce https://crbug.com/552697
	large = make([]byte, 64*1024+1)
	for i := 0; i < len(large); i++ {
		large[i] = byte(src.Uint32())
	}
}

func makeItems(contents ...[]byte) ([]*isolateservice.HandlersEndpointsV1Digest, [][]byte, map[isolated.HexDigest][]byte) {
	digests := make([]*isolateservice.HandlersEndpointsV1Digest, 0, len(contents))
	expected := make(map[isolated.HexDigest][]byte, len(contents))
	for _, content := range contents {
		hex := isolated.HashBytes(content)
		digests = append(digests, &isolateservice.HandlersEndpointsV1Digest{Digest: string(hex), IsIsolated: false, Size: int64(len(content))})
		expected[hex] = content
	}
	return digests, contents, expected
}

func testNormal(ctx context.Context, t *testing.T, contents ...[]byte) {
	digests, _, expected := makeItems(contents...)
	server := isolatedfake.New()
	ts := httptest.NewServer(server)
	defer ts.Close()
	client := New(nil, nil, ts.URL, DefaultNamespace, cantRetry)
	states, err := client.Contains(ctx, digests)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, len(digests), len(states))
	for _, state := range states {
		// The data is automatically compressed.
		err = client.Push(ctx, state, NewBytesSource(contents[state.status.Index]))
		ut.AssertEqual(t, nil, err)
	}
	ut.AssertEqual(t, nil, server.Error())
	ut.AssertEqual(t, expected, server.Contents())
	states, err = client.Contains(ctx, digests)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, len(digests), len(states))
	for _, state := range states {
		ut.AssertEqual(t, (*PushState)(nil), state)
	}
	ut.AssertEqual(t, nil, server.Error())
}

func testFlaky(ctx context.Context, t *testing.T, flake string) {
	server := isolatedfake.New()
	flaky := &killingMux{server: server, http503: map[string]int{flake: 10}}
	flaky.ts = httptest.NewServer(flaky)
	defer flaky.ts.Close()
	client := New(nil, nil, flaky.ts.URL, DefaultNamespace, fastRetry)

	digests, contents, expected := makeItems(foo, large)
	states, err := client.Contains(ctx, digests)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, len(digests), len(states))
	for _, state := range states {
		err = client.Push(ctx, state, NewBytesSource(contents[state.status.Index]))
		ut.AssertEqual(t, nil, err)
	}
	ut.AssertEqual(t, expected, server.Contents())
	states, err = client.Contains(ctx, digests)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, len(digests), len(states))
	for _, state := range states {
		ut.AssertEqual(t, (*PushState)(nil), state)
	}
	ut.AssertEqual(t, nil, server.Error())
	ut.AssertEqual(t, map[string]int{}, flaky.http503)
}

// killingMux inserts tears down connection in the middle of a transfer.
type killingMux struct {
	lock     sync.Mutex
	server   http.Handler
	http503  map[string]int // Number of bytes should be read before returning HTTP 500.
	tearDown map[string]int // Number of bytes should be read before killing the connection.
	ts       *httptest.Server
}

func readBytes(r io.Reader, toRead int) {
	b := make([]byte, toRead)
	read := 0
	for read < toRead {
		n, _ := r.Read(b[:toRead-read])
		read += n
	}
}

func (k *killingMux) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	f := func() bool {
		k.lock.Lock()
		defer k.lock.Unlock()
		if toRead, ok := k.http503[req.URL.Path]; ok {
			delete(k.http503, req.URL.Path)
			readBytes(req.Body, toRead)
			w.WriteHeader(503)
			return false
		}
		if toRead, ok := k.tearDown[req.URL.Path]; ok {
			delete(k.tearDown, req.URL.Path)
			readBytes(req.Body, toRead)
			k.ts.CloseClientConnections()
			return false
		}
		return true
	}
	if f() {
		log.Printf("%-4s %s", req.Method, req.URL.Path)
		k.server.ServeHTTP(w, req)
	}
}
