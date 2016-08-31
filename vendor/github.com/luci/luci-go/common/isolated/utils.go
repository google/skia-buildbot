// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package isolated

import (
	"encoding/hex"
	"hash"
	"io"
	"os"

	"github.com/luci/luci-go/common/api/isolate/isolateservice/v1"
)

// Sum is a shortcut to get a HexDigest from a hash.Hash.
func Sum(h hash.Hash) HexDigest {
	return HexDigest(hex.EncodeToString(h.Sum(nil)))
}

// Hash hashes a reader and returns a HexDigest from it.
func Hash(src io.Reader) (HexDigest, error) {
	h := GetHash()
	_, err := io.Copy(h, src)
	if err != nil {
		return HexDigest(""), err
	}
	return Sum(h), nil
}

// HashBytes hashes content and returns a HexDigest from it.
func HashBytes(content []byte) HexDigest {
	h := GetHash()
	_, _ = h.Write(content)
	return Sum(h)
}

// HashFile hashes a file and returns a HandlersEndpointsV1Digest out of it.
func HashFile(path string) (isolateservice.HandlersEndpointsV1Digest, error) {
	h := GetHash()
	f, err := os.Open(path)
	if err != nil {
		return isolateservice.HandlersEndpointsV1Digest{}, err
	}
	defer f.Close()
	size, err := io.Copy(h, f)
	if err != nil {
		return isolateservice.HandlersEndpointsV1Digest{}, err
	}
	return isolateservice.HandlersEndpointsV1Digest{Digest: string(Sum(h)), IsIsolated: false, Size: size}, nil
}
