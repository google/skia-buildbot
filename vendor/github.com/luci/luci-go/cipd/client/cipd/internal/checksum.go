// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package internal

import (
	"bytes"
	"crypto/sha1"
	"errors"

	"github.com/golang/protobuf/proto"

	"github.com/luci/luci-go/cipd/client/cipd/internal/messages"
)

// MarshalWithSHA1 serializes proto message to bytes, calculates SHA1 checksum
// of it, and returns serialized envelope that contains both.
//
// UnmarshalWithSHA1 can then be used to verify SHA1 and deserialized the
// original object.
func MarshalWithSHA1(pm proto.Message) ([]byte, error) {
	blob, err := proto.Marshal(pm)
	if err != nil {
		return nil, err
	}
	sum := sha1.Sum(blob)
	envelope := messages.BlobWithSHA1{Blob: blob, Sha1: sum[:]}
	return proto.Marshal(&envelope)
}

// UnmarshalWithSHA1 is reverse of MarshalWithSHA1.
//
// It checks SHA1 checksum and deserializes the object if it matches the blob.
func UnmarshalWithSHA1(buf []byte, pm proto.Message) error {
	envelope := messages.BlobWithSHA1{}
	if err := proto.Unmarshal(buf, &envelope); err != nil {
		return err
	}
	sum := sha1.Sum(envelope.Blob)
	if !bytes.Equal(sum[:], envelope.Sha1) {
		return errors.New("check sum of tag cache file is invalid")
	}
	return proto.Unmarshal(envelope.Blob, pm)
}
