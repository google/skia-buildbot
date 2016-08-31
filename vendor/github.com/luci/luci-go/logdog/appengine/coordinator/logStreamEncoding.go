// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package coordinator

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// encodedKeyPrefix is the prefix appended to encoded tag keys. See encodeKey
// for more information.
const encodedKeyPrefix = "Key_"

var keyEncoding = base64.URLEncoding.WithPadding('~')

// encodeKey encodes a key string in a manner that allows it to be used as a
// datastore property.
//
// Datastore properties may consist of [a-zA-Z0-9_]. However, filters on these
// properties require the "=" character to be used, so default base64
// padding is not acceptable.
func encodeKey(k string) string {
	return (encodedKeyPrefix + keyEncoding.EncodeToString([]byte(k)))
}

// decodeKey converts an encoded key into its original key string.
func decodeKey(k string) (string, error) {
	if !strings.HasPrefix(k, encodedKeyPrefix) {
		return "", fmt.Errorf("encoded key missing prefix (%s)", encodedKeyPrefix)
	}

	data, err := keyEncoding.DecodeString(k[len(encodedKeyPrefix):])
	if err != nil {
		return "", fmt.Errorf("failed to decode key: %v", err)
	}
	return string(data), nil
}
