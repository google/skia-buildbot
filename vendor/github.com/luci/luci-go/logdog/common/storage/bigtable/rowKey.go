// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bigtable

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"sync"

	"github.com/luci/luci-go/common/data/cmpbin"
)

// rowKeyBufferPool stores a pool of allocated Buffer instances for reuse when
// constructing row keys.
var (
	// errMalformedRowKey is an error that is returned if the row key in the
	// tables does not comform to our row key structure.
	errMalformedRowKey = errors.New("bigtable: malformed row key")

	// encodedPrefixSize is the size in bytes of the encoded row key prefix. All
	// rows from the same stream path share this prefix.
	encodedPrefixSize = base64.URLEncoding.EncodedLen(sha256.Size)
	// maxEncodedKeySize is the maximum size in bytes of a full row key.
	maxEncodedKeySize = encodedPrefixSize + (2 * (len("~") + hex.EncodedLen(cmpbin.MaxIntLen64)))

	rowKeyBufferPool = sync.Pool{
		New: func() interface{} {
			return &rowKeyBuffers{}
		},
	}
)

type rowKeyBuffers struct {
	// binBuf is a Buffer to write binary data for encoding.
	binBuf bytes.Buffer
	// key is where the encoded key get built.
	key []byte
	// size is the current number of bytes used in "key".
	size int
}

func withRowKeyBuffers(f func(rkb *rowKeyBuffers)) {
	rkb := rowKeyBufferPool.Get().(*rowKeyBuffers)
	defer rowKeyBufferPool.Put(rkb)

	rkb.reset()
	f(rkb)
}

func (rkb *rowKeyBuffers) reset() {
	if rkb.key == nil {
		rkb.key = make([]byte, maxEncodedKeySize)
	}
	rkb.size = 0
}

func (rkb *rowKeyBuffers) appendPathPrefix(pathHash []byte) {
	base64.URLEncoding.Encode(rkb.remaining(), pathHash)
	rkb.size += base64.URLEncoding.EncodedLen(len(pathHash))
}

func (rkb *rowKeyBuffers) appendInt64(i int64) {
	// Encode index to "cmpbin".
	rkb.binBuf.Reset()
	cmpbin.WriteInt(&rkb.binBuf, i)

	rkb.size += hex.Encode(rkb.remaining(), rkb.binBuf.Bytes())
}

func (rkb *rowKeyBuffers) appendBytes(d []byte) {
	rkb.size += copy(rkb.remaining(), d)
}

func (rkb *rowKeyBuffers) remaining() []byte {
	return rkb.key[rkb.size:]
}

func (rkb *rowKeyBuffers) value() string {
	return string(rkb.key[:rkb.size])
}

// rowKey is a BigTable row key.
//
// The row key is formed from a Path and its Index. The goal:
// - Rows with the same path should be clustered.
// - Rows with the same path should be sorted according to index.
//
// Since BigTable rows must be valid UTF8, and since paths are effectively
// unbounded, the row key will be formed by composing:
//
// [ base64(sha256(path)) ] + '~' + [ hex(cmpbin(index)) ] + '~' +
// [hex(cmpbin(count)]
type rowKey struct {
	pathHash []byte
	index    int64
	count    int64
}

// newRowKey generates the row key matching a given entry path and index.
func newRowKey(project, path string, index, count int64) *rowKey {
	h := sha256.New()

	_, _ = h.Write([]byte(project))
	_, _ = h.Write([]byte("/"))
	_, _ = h.Write([]byte(path))
	return &rowKey{
		pathHash: h.Sum(nil),
		index:    index,
		count:    count,
	}
}

// decodeRowKey decodes an encoded row key into its structural components.
func decodeRowKey(v string) (*rowKey, error) {
	keyParts := strings.SplitN(v, "~", 3)
	if len(keyParts) != 3 {
		return nil, errMalformedRowKey
	}

	hashEnc, idxEnc, countEnc := keyParts[0], keyParts[1], keyParts[2]
	if base64.URLEncoding.DecodedLen(len(hashEnc)) < sha256.Size {
		return nil, errMalformedRowKey
	}

	// Decode encoded project/path hash.
	var err error
	rk := rowKey{}
	rk.pathHash, err = base64.URLEncoding.DecodeString(hashEnc)
	if err != nil {
		return nil, errMalformedRowKey
	}

	// Decode index.
	rk.index, err = readHexInt64(idxEnc)
	if err != nil {
		return nil, err
	}

	// If a count is available, decode that as well.
	rk.count, err = readHexInt64(countEnc)
	if err != nil {
		return nil, err
	}

	return &rk, nil
}

func (rk *rowKey) String() string {
	return rk.encode()
}

// newRowKey instantiates a new rowKey from its components.
func (rk *rowKey) encode() (v string) {
	// Write the final key to "key": (base64(HASH)~hex(INDEX))
	withRowKeyBuffers(func(rkb *rowKeyBuffers) {
		rkb.appendPathPrefix(rk.pathHash)
		rkb.appendBytes([]byte("~"))
		rkb.appendInt64(rk.index)
		rkb.appendBytes([]byte("~"))
		rkb.appendInt64(rk.count)
		v = rkb.value()
	})
	return
}

// prefix returns the encoded path prefix for the row key, which is the hash of
// that row's project/path.
func (rk *rowKey) pathPrefix() (v string) {
	withRowKeyBuffers(func(rkb *rowKeyBuffers) {
		rkb.appendPathPrefix(rk.pathHash)
		rkb.appendBytes([]byte("~"))
		v = rkb.value()
	})
	return
}

// pathPrefixUpperBound returns the path prefix that is higher than any path
// allowed in the row key space.
//
// This is accomplished by appending a "~" character to the path prefix,
// creating something like this:
//
//     prefix~~
//
// The "prefix~" is shared with all keys in "rk", but the extra "~" is larger
// than any hex-encoded row index, so this key will always be larger.
func (rk *rowKey) pathPrefixUpperBound() (v string) {
	withRowKeyBuffers(func(rkb *rowKeyBuffers) {
		rkb.appendPathPrefix(rk.pathHash)
		rkb.appendBytes([]byte("~~"))
		v = rkb.value()
	})
	return
}

// sharesPrefixWith tests if the "path" component of the row key "rk" matches
// the "path" component of "o".
func (rk *rowKey) sharesPathWith(o *rowKey) bool {
	return bytes.Equal(rk.pathHash, o.pathHash)
}

func readHexInt64(v string) (int64, error) {
	d, err := hex.DecodeString(v)
	if err != nil {
		return 0, errMalformedRowKey
	}

	dr := bytes.NewReader(d)
	value, _, err := cmpbin.ReadInt(dr)
	if err != nil {
		return 0, errMalformedRowKey
	}

	// There should be no more data.
	if dr.Len() > 0 {
		return 0, errMalformedRowKey
	}

	return value, nil
}
