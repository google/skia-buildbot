// Copyright 2025 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chromeperf

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMinimumViableSetOfRequiredFields_OK(t *testing.T) {
	tools := GetTools(nil)
	require.Equal(t, 4, len(tools))
}
