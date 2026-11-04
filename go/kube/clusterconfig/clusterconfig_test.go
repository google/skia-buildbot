// Copyright 2022 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package clusterconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewFromEmbeddedConfig_HappyPath(t *testing.T) {

	cfg, err := NewFromEmbeddedConfig()
	require.NoError(t, err)
	require.Equal(t, "https://skia.googlesource.com/k8s-config", cfg.Repo)
}
