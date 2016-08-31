// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package isolate

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luci/luci-go/client/archiver"
	"github.com/luci/luci-go/client/internal/common"
	"github.com/luci/luci-go/common/api/isolate/isolateservice/v1"
	"github.com/luci/luci-go/common/data/text/units"
	"github.com/luci/luci-go/common/isolated"
	"github.com/luci/luci-go/common/isolatedclient"
	"github.com/luci/luci-go/common/isolatedclient/isolatedfake"
	"github.com/maruel/ut"
)

func init() {
	log.SetOutput(ioutil.Discard)
}

func TestReplaceVars(t *testing.T) {
	t.Parallel()

	opts := &ArchiveOptions{PathVariables: map[string]string{"VAR": "wonderful"}}

	// Single replacement.
	r, err := ReplaceVariables("hello <(VAR) world", opts)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, "hello wonderful world", r)

	// Multiple replacement.
	r, err = ReplaceVariables("hello <(VAR) <(VAR) world", opts)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, "hello wonderful wonderful world", r)

	// Replacement of missing variable.
	r, err = ReplaceVariables("hello <(MISSING) world", opts)
	ut.AssertEqual(t, "no value for variable 'MISSING'", err.Error())
}

func TestArchive(t *testing.T) {
	// Create a .isolate file and archive it.
	t.Parallel()
	server := isolatedfake.New()
	ts := httptest.NewServer(server)
	defer ts.Close()
	a := archiver.New(isolatedclient.New(nil, nil, ts.URL, isolatedclient.DefaultNamespace, nil), nil)

	// Setup temporary directory.
	//   /base/bar
	//   /base/ignored
	//   /foo/baz.isolate
	//   /link -> /base/bar
	//   /second/boz
	// Result:
	//   /baz.isolated
	tmpDir, err := ioutil.TempDir("", "isolate")
	ut.AssertEqual(t, nil, err)
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Fail()
		}
	}()
	baseDir := filepath.Join(tmpDir, "base")
	fooDir := filepath.Join(tmpDir, "foo")
	secondDir := filepath.Join(tmpDir, "second")
	ut.AssertEqual(t, nil, os.Mkdir(baseDir, 0700))
	ut.AssertEqual(t, nil, os.Mkdir(fooDir, 0700))
	ut.AssertEqual(t, nil, os.Mkdir(secondDir, 0700))
	ut.AssertEqual(t, nil, ioutil.WriteFile(filepath.Join(baseDir, "bar"), []byte("foo"), 0600))
	ut.AssertEqual(t, nil, ioutil.WriteFile(filepath.Join(baseDir, "ignored"), []byte("ignored"), 0600))
	ut.AssertEqual(t, nil, ioutil.WriteFile(filepath.Join(secondDir, "boz"), []byte("foo2"), 0600))
	isolate := `{
		'variables': {
			'files': [
				'../base/',
				'../second/',
				'../link',
			],
		},
		'conditions': [
			['OS=="amiga"', {
				'variables': {
					'command': ['amiga', '<(EXTRA)'],
				},
			}],
			['OS=="win"', {
				'variables': {
					'command': ['win'],
				},
			}],
		],
	}`
	isolatePath := filepath.Join(fooDir, "baz.isolate")
	ut.AssertEqual(t, nil, ioutil.WriteFile(isolatePath, []byte(isolate), 0600))
	if !common.IsWindows() {
		ut.AssertEqual(t, nil, os.Symlink(filepath.Join("base", "bar"), filepath.Join(tmpDir, "link")))
	} else {
		ut.AssertEqual(t, nil, ioutil.WriteFile(filepath.Join(tmpDir, "link"), []byte("no link on Windows"), 0600))
	}
	opts := &ArchiveOptions{
		Isolate:         isolatePath,
		Isolated:        filepath.Join(tmpDir, "baz.isolated"),
		Blacklist:       common.Strings{"ignored", "*.isolate"},
		PathVariables:   map[string]string{"VAR": "wonderful"},
		ExtraVariables:  map[string]string{"EXTRA": "really"},
		ConfigVariables: map[string]string{"OS": "amiga"},
	}
	item := Archive(a, opts)
	ut.AssertEqual(t, "baz.isolated", item.DisplayName)
	item.WaitForHashed()
	ut.AssertEqual(t, nil, item.Error())
	ut.AssertEqual(t, nil, a.Close())

	mode := 0600
	if common.IsWindows() {
		mode = 0666
	}

	//   /base/
	baseIsolatedData := isolated.Isolated{
		Algo: "sha-1",
		Files: map[string]isolated.File{
			filepath.Join("base", "bar"): {Digest: "0beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33", Mode: newInt(mode), Size: newInt64(3)},
		},
		Version: isolated.IsolatedFormatVersion,
	}
	encoded, err := json.Marshal(baseIsolatedData)
	ut.AssertEqual(t, nil, err)
	baseIsolatedEncoded := string(encoded) + "\n"
	baseIsolatedHash := isolated.HashBytes([]byte(baseIsolatedEncoded))

	//   /second/
	secondIsolatedData := isolated.Isolated{
		Algo: "sha-1",
		Files: map[string]isolated.File{
			filepath.Join("second", "boz"): {Digest: "aaadd94977b8fbf3f6fb09fc3bbbc9edbdfa8427", Mode: newInt(mode), Size: newInt64(4)},
		},
		Version: isolated.IsolatedFormatVersion,
	}
	encoded, err = json.Marshal(secondIsolatedData)
	ut.AssertEqual(t, nil, err)
	secondIsolatedEncoded := string(encoded) + "\n"
	secondIsolatedHash := isolated.HashBytes([]byte(secondIsolatedEncoded))

	isolatedData := isolated.Isolated{
		Algo:    "sha-1",
		Command: []string{"amiga", "really"},
		Files:   map[string]isolated.File{},
		// This list must be in deterministic order.
		Includes:    isolated.HexDigests{baseIsolatedHash, secondIsolatedHash},
		RelativeCwd: "foo",
		Version:     isolated.IsolatedFormatVersion,
	}
	if !common.IsWindows() {
		isolatedData.Files["link"] = isolated.File{Link: newString(filepath.Join("base", "bar"))}
	} else {
		isolatedData.Files["link"] = isolated.File{Digest: "12339b9756c2994f85c310d560bc8c142a6b79a1", Mode: newInt(0666), Size: newInt64(18)}
	}
	encoded, err = json.Marshal(isolatedData)
	ut.AssertEqual(t, nil, err)
	isolatedEncoded := string(encoded) + "\n"
	isolatedHash := isolated.HashBytes([]byte(isolatedEncoded))

	expected := map[string]string{
		"0beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33": "foo",
		"aaadd94977b8fbf3f6fb09fc3bbbc9edbdfa8427": "foo2",
		string(baseIsolatedHash):                   baseIsolatedEncoded,
		string(isolatedHash):                       isolatedEncoded,
		string(secondIsolatedHash):                 secondIsolatedEncoded,
	}
	if common.IsWindows() {
		expected["12339b9756c2994f85c310d560bc8c142a6b79a1"] = "no link on Windows"
	}
	actual := map[string]string{}
	for k, v := range server.Contents() {
		actual[string(k)] = string(v)
		ut.AssertEqualf(t, expected[string(k)], actual[string(k)], "%s: %#v", k, actual[string(k)])
	}
	ut.AssertEqual(t, expected, actual)
	ut.AssertEqual(t, isolatedHash, item.Digest())

	stats := a.Stats()
	ut.AssertEqual(t, 0, stats.TotalHits())
	ut.AssertEqual(t, units.Size(0), stats.TotalBytesHits())
	size := 3 + 4 + len(baseIsolatedEncoded) + len(isolatedEncoded) + len(secondIsolatedEncoded)
	if !common.IsWindows() {
		ut.AssertEqual(t, 5, stats.TotalMisses())
		ut.AssertEqual(t, units.Size(size), stats.TotalBytesPushed())
	} else {
		ut.AssertEqual(t, 6, stats.TotalMisses())
		// Includes the duplicate due to lack of symlink.
		ut.AssertEqual(t, units.Size(size+18), stats.TotalBytesPushed())
	}

	ut.AssertEqual(t, nil, server.Error())
	digest, err := isolated.HashFile(filepath.Join(tmpDir, "baz.isolated"))
	ut.AssertEqual(t, isolateservice.HandlersEndpointsV1Digest{Digest: string(isolatedHash), IsIsolated: false, Size: int64(len(isolatedEncoded))}, digest)
	ut.AssertEqual(t, nil, err)
}

// Test that if the isolate file is not found, the error is properly propagated.
func TestArchiveFileNotFoundReturnsError(t *testing.T) {
	t.Parallel()
	a := archiver.New(isolatedclient.New(nil, nil, "http://unused", isolatedclient.DefaultNamespace, nil), nil)
	opts := &ArchiveOptions{
		Isolate:  "/this-file-does-not-exist",
		Isolated: "/this-file-doesnt-either",
	}
	item := Archive(a, opts)
	item.WaitForHashed()
	err := item.Error()
	ut.AssertEqual(t, true, strings.HasPrefix(err.Error(), "open /this-file-does-not-exist: "))
	closeErr := a.Close()
	ut.AssertEqual(t, true, closeErr != nil)
	ut.AssertEqual(t, true, strings.HasPrefix(closeErr.Error(), "open /this-file-does-not-exist: "))
}
