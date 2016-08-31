// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package coordinator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// HashID is a hex-encoded SHA256 hash.
type HashID string

const validHashIDChars = "0123456789abcdef"

func makeHashID(v string) HashID {
	hash := sha256.Sum256([]byte(v))
	return HashID(hex.EncodeToString(hash[:]))
}

// Normalize normalizes the hash ID and verifies its integrity.
func (id *HashID) Normalize() error {
	// encoding/hex encodes using lower-case hexadecimal. Note that this is a
	// no-op if the ID is already lowercase.
	idv := strings.ToLower(string(*id))

	if decodeSize := hex.DecodedLen(len(idv)); decodeSize != sha256.Size {
		return fmt.Errorf("invalid SHA256 hash size (%d != %d)", decodeSize, sha256.Size)
	}
	for i, r := range idv {
		if !strings.ContainsRune(validHashIDChars, r) {
			return fmt.Errorf("invalid character '%c' at %d", r, i)
		}
	}
	*id = HashID(idv)
	return nil
}
