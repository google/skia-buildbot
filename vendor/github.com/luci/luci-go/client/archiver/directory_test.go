// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package archiver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/luci/luci-go/client/internal/common"
	"github.com/luci/luci-go/common/data/text/units"
	"github.com/luci/luci-go/common/isolated"
	"github.com/luci/luci-go/common/isolatedclient"
	"github.com/luci/luci-go/common/isolatedclient/isolatedfake"
	"github.com/maruel/ut"
)

func TestWalkInexistent(t *testing.T) {
	ch := make(chan *walkItem)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(ch)
		walk("inexistent_directory", nil, ch)
	}()
	item := <-ch
	osErr := "lstat inexistent_directory: no such file or directory"
	if common.IsWindows() {
		osErr = "GetFileAttributesEx inexistent_directory: The system cannot find the file specified."
	}
	err := fmt.Errorf("walk(inexistent_directory): %s", osErr)
	ut.AssertEqual(t, &walkItem{err: err}, item)
	item, ok := <-ch
	ut.AssertEqual(t, (*walkItem)(nil), item)
	ut.AssertEqual(t, false, ok)
	wg.Wait()
}

func TestWalkBadRegexp(t *testing.T) {
	ch := make(chan *walkItem)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(ch)
		walk("inexistent", []string{"a["}, ch)
	}()
	item := <-ch
	ut.AssertEqual(t, &walkItem{err: errors.New("bad blacklist pattern \"a[\"")}, item)
	item, ok := <-ch
	ut.AssertEqual(t, (*walkItem)(nil), item)
	ut.AssertEqual(t, false, ok)
	wg.Wait()
}

func TestPushDirectory(t *testing.T) {
	// Uploads a real directory. 2 times the same file.
	t.Parallel()
	server := isolatedfake.New()
	ts := httptest.NewServer(server)
	defer ts.Close()
	a := New(isolatedclient.New(nil, nil, ts.URL, isolatedclient.DefaultNamespace, nil), nil)

	// Setup temporary directory.
	tmpDir, err := ioutil.TempDir("", "archiver")
	ut.AssertEqual(t, nil, err)
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Fail()
		}
	}()
	baseDir := filepath.Join(tmpDir, "base")
	ignoredDir := filepath.Join(tmpDir, "ignored1")
	ut.AssertEqual(t, nil, os.Mkdir(baseDir, 0700))
	ut.AssertEqual(t, nil, ioutil.WriteFile(filepath.Join(baseDir, "bar"), []byte("foo"), 0600))
	ut.AssertEqual(t, nil, ioutil.WriteFile(filepath.Join(baseDir, "bar_dupe"), []byte("foo"), 0600))
	if !common.IsWindows() {
		ut.AssertEqual(t, nil, os.Symlink("bar", filepath.Join(baseDir, "link")))
	}
	ut.AssertEqual(t, nil, ioutil.WriteFile(filepath.Join(baseDir, "ignored2"), []byte("ignored"), 0600))
	ut.AssertEqual(t, nil, os.Mkdir(ignoredDir, 0700))
	ut.AssertEqual(t, nil, ioutil.WriteFile(filepath.Join(ignoredDir, "really"), []byte("ignored"), 0600))

	item := PushDirectory(a, tmpDir, "", []string{"ignored1", filepath.Join("*", "ignored2")})
	ut.AssertEqual(t, filepath.Base(tmpDir)+".isolated", item.DisplayName)
	item.WaitForHashed()
	ut.AssertEqual(t, nil, a.Close())

	mode := 0600
	if common.IsWindows() {
		mode = 0666
	}
	isolatedData := isolated.Isolated{
		Algo: "sha-1",
		Files: map[string]isolated.File{
			filepath.Join("base", "bar"):      {Digest: "0beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33", Mode: newInt(mode), Size: newInt64(3)},
			filepath.Join("base", "bar_dupe"): {Digest: "0beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33", Mode: newInt(mode), Size: newInt64(3)},
		},
		Version: isolated.IsolatedFormatVersion,
	}
	if !common.IsWindows() {
		isolatedData.Files[filepath.Join("base", "link")] = isolated.File{Link: newString("bar")}
	}
	encoded, err := json.Marshal(isolatedData)
	ut.AssertEqual(t, nil, err)
	isolatedEncoded := string(encoded) + "\n"
	isolatedHash := isolated.HashBytes([]byte(isolatedEncoded))

	expected := map[string]string{
		"0beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33": "foo",
		string(isolatedHash):                       isolatedEncoded,
	}
	actual := map[string]string{}
	for k, v := range server.Contents() {
		actual[string(k)] = string(v)
	}
	ut.AssertEqual(t, expected, actual)
	ut.AssertEqual(t, isolatedHash, item.Digest())

	stats := a.Stats()
	ut.AssertEqual(t, 0, stats.TotalHits())
	// There're 3 cache misses even if the same content is looked up twice.
	ut.AssertEqual(t, 3, stats.TotalMisses())
	ut.AssertEqual(t, units.Size(0), stats.TotalBytesHits())
	ut.AssertEqual(t, units.Size(3+3+len(isolatedEncoded)), stats.TotalBytesPushed())

	ut.AssertEqual(t, nil, server.Error())
}
