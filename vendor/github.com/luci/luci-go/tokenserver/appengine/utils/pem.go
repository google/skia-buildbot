// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package utils

import (
	"encoding/pem"
	"fmt"
)

// ParsePEM takes pem-encoded block and decodes it, checking the header.
func ParsePEM(data, header string) ([]byte, error) {
	block, rest := pem.Decode([]byte(data))
	if len(rest) != 0 || block == nil {
		return nil, fmt.Errorf("not a valid %q PEM", header)
	}
	if block.Type != header {
		return nil, fmt.Errorf("expecting %q, got %q", header, block.Type)
	}
	return block.Bytes, nil
}

// DumpPEM transforms block to pem-encoding.
//
// Reverse of ParsePEM.
func DumpPEM(data []byte, header string) string {
	return string(pem.EncodeToMemory(&pem.Block{
		Type:  header,
		Bytes: data,
	}))
}
