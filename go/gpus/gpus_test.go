// Copyright 2022 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gpus

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGPUVendorNameToID_CanonicalizesCase(t *testing.T) {
	assert.Equal(t, VendorNameToID("Nvidia"), VendorID("10de"))
}
