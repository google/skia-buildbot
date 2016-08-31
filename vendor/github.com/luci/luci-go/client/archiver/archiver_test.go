// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package archiver

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/luci/luci-go/client/internal/common"
	"github.com/luci/luci-go/common/data/text/units"
	"github.com/luci/luci-go/common/isolated"
	"github.com/luci/luci-go/common/isolatedclient"
	"github.com/luci/luci-go/common/isolatedclient/isolatedfake"
	"github.com/maruel/ut"
)

func init() {
	log.SetOutput(ioutil.Discard)
}

func TestArchiverEmpty(t *testing.T) {
	t.Parallel()
	a := New(isolatedclient.New(nil, nil, "https://localhost:1", isolatedclient.DefaultNamespace, nil), nil)
	stats := a.Stats()
	ut.AssertEqual(t, 0, stats.TotalHits())
	ut.AssertEqual(t, 0, stats.TotalMisses())
	ut.AssertEqual(t, units.Size(0), stats.TotalBytesHits())
	ut.AssertEqual(t, units.Size(0), stats.TotalBytesPushed())
	ut.AssertEqual(t, nil, a.Close())
}

func TestArchiverFile(t *testing.T) {
	t.Parallel()
	server := isolatedfake.New()
	ts := httptest.NewServer(server)
	defer ts.Close()
	a := New(isolatedclient.New(nil, nil, ts.URL, isolatedclient.DefaultNamespace, nil), nil)

	fEmpty, err := ioutil.TempFile("", "archiver")
	ut.AssertEqual(t, nil, err)
	item1 := a.PushFile(fEmpty.Name(), fEmpty.Name(), 0)
	ut.AssertEqual(t, fEmpty.Name(), item1.DisplayName)
	fFoo, err := ioutil.TempFile("", "archiver")
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, nil, ioutil.WriteFile(fFoo.Name(), []byte("foo"), 0600))
	item2 := a.PushFile(fFoo.Name(), fFoo.Name(), 0)
	// Push the same file another time. It'll get linked to the first.
	item3 := a.PushFile(fFoo.Name(), fFoo.Name(), 0)
	item1.WaitForHashed()
	item2.WaitForHashed()
	item3.WaitForHashed()
	ut.AssertEqual(t, nil, a.Close())

	stats := a.Stats()
	ut.AssertEqual(t, 0, stats.TotalHits())
	// Only 2 lookups, not 3.
	ut.AssertEqual(t, 2, stats.TotalMisses())
	ut.AssertEqual(t, units.Size(0), stats.TotalBytesHits())
	ut.AssertEqual(t, units.Size(3), stats.TotalBytesPushed())
	expected := map[isolated.HexDigest][]byte{
		"0beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33": []byte("foo"),
		"da39a3ee5e6b4b0d3255bfef95601890afd80709": {},
	}
	ut.AssertEqual(t, expected, server.Contents())
	ut.AssertEqual(t, isolated.HexDigest("da39a3ee5e6b4b0d3255bfef95601890afd80709"), item1.Digest())
	ut.AssertEqual(t, nil, item1.Error())
	ut.AssertEqual(t, isolated.HexDigest("0beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33"), item2.Digest())
	ut.AssertEqual(t, nil, item2.Error())
	ut.AssertEqual(t, isolated.HexDigest("0beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33"), item3.Digest())
	ut.AssertEqual(t, nil, item3.Error())
	ut.AssertEqual(t, nil, server.Error())
}

func TestArchiverFileHit(t *testing.T) {
	t.Parallel()
	server := isolatedfake.New()
	ts := httptest.NewServer(server)
	defer ts.Close()
	a := New(isolatedclient.New(nil, nil, ts.URL, isolatedclient.DefaultNamespace, nil), nil)
	server.Inject([]byte("foo"))
	item := a.Push("foo", isolatedclient.NewBytesSource([]byte("foo")), 0)
	item.WaitForHashed()
	ut.AssertEqual(t, isolated.HexDigest("0beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33"), item.Digest())
	ut.AssertEqual(t, nil, a.Close())

	stats := a.Stats()
	ut.AssertEqual(t, 1, stats.TotalHits())
	ut.AssertEqual(t, 0, stats.TotalMisses())
	ut.AssertEqual(t, units.Size(3), stats.TotalBytesHits())
	ut.AssertEqual(t, units.Size(0), stats.TotalBytesPushed())
}

func TestArchiverCancel(t *testing.T) {
	t.Parallel()
	server := isolatedfake.New()
	ts := httptest.NewServer(server)
	defer ts.Close()
	a := New(isolatedclient.New(nil, nil, ts.URL, isolatedclient.DefaultNamespace, nil), nil)

	tmpDir, err := ioutil.TempDir("", "archiver")
	ut.AssertEqual(t, nil, err)
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Fail()
		}
	}()

	// This will trigger an eventual Cancel().
	nonexistent := filepath.Join(tmpDir, "nonexistent")
	item1 := a.PushFile("foo", nonexistent, 0)
	ut.AssertEqual(t, "foo", item1.DisplayName)

	fileName := filepath.Join(tmpDir, "existent")
	ut.AssertEqual(t, nil, ioutil.WriteFile(fileName, []byte("foo"), 0600))
	item2 := a.PushFile("existent", fileName, 0)
	item1.WaitForHashed()
	item2.WaitForHashed()
	osErr := "no such file or directory"
	if common.IsWindows() {
		osErr = "The system cannot find the file specified."
	}
	expected := fmt.Errorf("source(foo) failed: open %s: %s\n", nonexistent, osErr)
	ut.AssertEqual(t, expected, <-a.Channel())
	ut.AssertEqual(t, expected, a.Close())
	ut.AssertEqual(t, nil, server.Error())
}

func TestArchiverPushClosed(t *testing.T) {
	t.Parallel()
	a := New(nil, nil)
	ut.AssertEqual(t, nil, a.Close())
	ut.AssertEqual(t, (*Item)(nil), a.PushFile("ignored", "ignored", 0))
}
