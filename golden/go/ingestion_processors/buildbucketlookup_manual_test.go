// Copyright 2021 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ingestion_processors

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/httputils"
)

func TestBuildbucketLookup_ValidTJID_Success(t *testing.T) {

	client := httputils.DefaultClientConfig().Client()
	bbClient := buildbucket.NewClient(client)
	bc := newBuildbucketLookupClient(bbClient)

	crsName, clID, psOrder, err := bc.Lookup(context.Background(), "8851407306039688080")
	require.NoError(t, err)
	require.Equal(t, "gerrit", crsName)
	require.Equal(t, "389927", clID)
	require.Equal(t, 1, psOrder)
}
