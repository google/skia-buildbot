// Package scrap defines the scrap types and functions on them.
package scrap

import (
	"bytes"
	"context"
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

	var b bytes.Buffer

	s.On("FileWriter", testutils.AnyContext, fmt.Sprintf("svg/%s", svgHash), gcs.FileWriteOptions{ContentEncoding: "gzip", ContentType: "application/json"}).Return(&myWriteCloser{b})

	se := New(s)
	id, err := se.CreateScrap(context.Background(), ScrapBody{
		Type: SVG,
		Body: "<svg></svg>",
	})
	require.NoError(t, err)
	require.Equal(t, ScrapID{Hash: svgHash}, id)
}
