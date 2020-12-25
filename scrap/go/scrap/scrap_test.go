// Package scrap defines the scrap types and functions on them.
package scrap

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/test_gcsclient"
	"go.skia.org/infra/go/testutils"
)

const svgHash = "975c5b1ad481d0e8b0875e03b0bf2b375da68593d8beac607f23f40b2e53ca31"

type myWriteCloser struct {
	bytes.Buffer
}

func (*myWriteCloser) Close() error {
	return nil
}

func TestCreateScrap_HappyPath_Success(t *testing.T) {
	s := &test_gcsclient.GCSClient{}

	var w myWriteCloser

	s.On("FileWriter", testutils.AnyContext, fmt.Sprintf("svg/%s", svgHash),
		gcs.FileWriteOptions{
			ContentEncoding: "gzip",
			ContentType:     "application/json",
		}).Return(&w)

	se := New(s)
	sentBody := ScrapBody{
		Type: SVG,
		Body: "<svg></svg>",
	}
	id, err := se.CreateScrap(context.Background(), sentBody)
	require.NoError(t, err)

	require.Equal(t, ScrapID{Hash: svgHash}, id)

	// Unzip and decode the written body to confirm it matches what we sent.
	r := bytes.NewReader(w.Bytes())
	rz, err := gzip.NewReader(r)
	require.NoError(t, err)
	var storedBody ScrapBody
	err = json.NewDecoder(rz).Decode(&storedBody)
	require.NoError(t, err)
	require.Equal(t, storedBody, sentBody)
}
