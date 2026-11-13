// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package insights

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
)

const (
	pageTokenVersion = 1

	// maxPageOffset bounds the offset a page token can carry.
	maxPageOffset = 1000000

	// maxPageTokenLength bounds the size of a page token accepted as input.
	maxPageTokenLength = 256
)

// pageToken is the decoded form of the opaque page_token (contract C2).
//
// The store only supports offset pagination today, so the token carries the offset of the next
// row to scan. Clients must treat the token as opaque, which lets the server switch to keyset
// pagination (FIX-12) without breaking them. The token is not signed: tampering with it can only
// change which rows of the caller's own read-only query are returned.
type pageToken struct {
	V int    `json:"v"`
	O int    `json:"o"`
	F string `json:"f"`
}

// filterHash returns a short hash of the filter parameters, so a token can only be used with
// the filters it was issued for.
func filterHash(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(h[:8])
}

// encodePageToken returns an opaque page token.
func encodePageToken(offset int, filter string) string {
	b, err := json.Marshal(pageToken{V: pageTokenVersion, O: offset, F: filter})
	if err != nil {
		// Marshaling a struct of ints and strings cannot fail.
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// decodePageToken decodes a page token issued for the given filter. An empty token means the
// first page.
func decodePageToken(token, filter string) (int, error) {
	if token == "" {
		return 0, nil
	}
	invalid := InvalidArgument(
		"pass next_page_token unchanged with the same filters as the previous call, or omit it to start from the first page",
		"invalid page_token")
	if len(token) > maxPageTokenLength {
		return 0, invalid
	}
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, invalid
	}
	var t pageToken
	if err := json.Unmarshal(b, &t); err != nil {
		return 0, invalid
	}
	if t.V != pageTokenVersion || t.O < 0 || t.O > maxPageOffset || t.F != filter {
		return 0, invalid
	}
	return t.O, nil
}
