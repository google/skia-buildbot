// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package shards provides a low level support for implementing sharded set of
// []byte blobs.
package shards

import (
	"bytes"
	"encoding/gob"
	"hash/fnv"
	"sort"
)

// Shard is a set of byte blobs.
//
// It represents s single shard of a sharded set.
type Shard map[string]struct{}

// ParseShard deserializes a shard (serialized with Serialize).
func ParseShard(blob []byte) (Shard, error) {
	var sorted []string
	dec := gob.NewDecoder(bytes.NewReader(blob))
	if err := dec.Decode(&sorted); err != nil {
		return nil, err
	}
	shard := make(Shard, len(sorted))
	for _, blob := range sorted {
		shard[blob] = struct{}{}
	}
	return shard, nil
}

// Serialize serializes the shard to a byte buffer.
func (s Shard) Serialize() []byte {
	sorted := make([]string, 0, len(s))
	for blob := range s {
		sorted = append(sorted, blob)
	}
	sort.Strings(sorted)
	out := bytes.Buffer{}
	enc := gob.NewEncoder(&out)
	err := enc.Encode(sorted)
	if err != nil {
		panic("impossible error when encoding []string")
	}
	return out.Bytes()
}

// Set is an array of shards (representing a single sharded set).
//
// The size of the array is number of shards in the set. Allocate it using
// regular make(...).
type Set []Shard

// Insert adds a blob into the sharded set.
func (s Set) Insert(blob []byte) {
	shard := &s[ShardIndex(blob, len(s))]
	if *shard == nil {
		*shard = make(Shard)
	}
	(*shard)[string(blob)] = struct{}{}
}

// ShardIndex returns an index of a shard to use when storing given blob.
func ShardIndex(member []byte, shardCount int) int {
	hash := fnv.New32()
	hash.Write(member)
	return int(hash.Sum32() % uint32(shardCount))
}
