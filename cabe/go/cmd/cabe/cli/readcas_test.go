// Copyright 2023 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadCASCommand(t *testing.T) {
	got := ReadCASCommand()
	require.NotNil(t, got)
	assert.Equal(t, "readcas", got.Name)
}
