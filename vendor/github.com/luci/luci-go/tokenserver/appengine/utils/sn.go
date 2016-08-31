// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package utils

import (
	"fmt"
	"math/big"
)

// SerializeSN converts a certificate serial number to a byte blob.
func SerializeSN(sn *big.Int) ([]byte, error) {
	blob, err := sn.GobEncode()
	if err != nil {
		return nil, fmt.Errorf("can't encode SN - %s", err)
	}
	return blob, nil
}
