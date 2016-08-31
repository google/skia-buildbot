// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/golang/protobuf/jsonpb"
	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/logging"

	"github.com/luci/luci-go/tokenserver/api"
)

// stateInToken is stored in the token file in 'tokend_state' field.
//
// The content of this struct is private implementation detail of tokend, so
// we don't bother with proto serialization and use simpler JSON.
type stateInToken struct {
	InputsDigest string // digest of parameters used to generate the token
	Version      string // version of the daemon that produced the token
}

// readTokenFile reads the token file from disk.
//
// In case of problems, logs errors and returns default structs.
func readTokenFile(ctx context.Context, path string) (*tokenserver.TokenFile, *stateInToken) {
	blob, err := ioutil.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logging.Warningf(ctx, "Failed to read token file from %q - %s", path, err)
		}
		return &tokenserver.TokenFile{}, &stateInToken{}
	}
	out := &tokenserver.TokenFile{}
	if err = jsonpb.Unmarshal(bytes.NewReader(blob), out); err != nil {
		logging.Warningf(ctx, "Failed to unmarshal token file %q - %s", path, err)
		return &tokenserver.TokenFile{}, &stateInToken{}
	}
	// Attempt to decode stateInToken. Ignore if doesn't work, no big deal.
	state := &stateInToken{}
	if err = json.Unmarshal(out.TokendState, state); err != nil {
		logging.Warningf(ctx, "Failed to unmarshal tokend_state - %s", err)
		*state = stateInToken{}
	}
	return out, state
}

// writeTokenFile replaces the token file on disk.
//
// It updates tokenFile.TokendState field first, then serializes the token and
// dumps it on disk.
//
// It replaces the token file atomically, retrying a bunch of times in case of
// concurrent access errors (important on Windows).
//
// The token file is world-readable (0644 permissions).
func writeTokenFile(ctx context.Context, tokenFile *tokenserver.TokenFile, state *stateInToken, path string) error {
	stateBlob, err := json.Marshal(state)
	if err != nil {
		logging.Errorf(ctx, "Failed to marshal tokend_state - %s", err)
		return err
	}
	tokenFile.TokendState = stateBlob

	out := bytes.Buffer{}
	m := jsonpb.Marshaler{}
	m.Indent = "  "
	if err := m.Marshal(&out, tokenFile); err != nil {
		logging.Errorf(ctx, "Failed to marshal the token file - %s", err)
		return err
	}
	blob := out.Bytes()

	return AtomicWriteFile(ctx, path, blob, 0644)
}
