// Package scrap defines the scrap types and functions on them.
package scrap

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/test_gcsclient"
	"go.skia.org/infra/go/testutils"
)

const svgHash = "975c5b1ad481d0e8b0875e03b0bf2b375da68593d8beac607f23f40b2e53ca31"

const scrapName = "@MyScrapName"

// myWriteCloser is a wrapper that turns a bytes.Buffer from an io.Writer to an io.WriteCloser.
type myWriteCloser struct {
	bytes.Buffer
}

func (*myWriteCloser) Close() error {
	return nil
}

// myErrorOnWriteWriteCloser turns a bytes.Buffer from an io.Writer to an io.WriteCloser that errors when Writing.
type myErrorOnWriteWriteCloser struct {
	bytes.Buffer
}

func (*myErrorOnWriteWriteCloser) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func (*myErrorOnWriteWriteCloser) Close() error {
	return nil
}

// myErrorOnWriteWriteCloser turns a bytes.Buffer from an io.Writer to an io.WriteCloser that errors when Closing.
type myErrorOnCloseWriteCloser struct {
	bytes.Buffer
}

func (*myErrorOnCloseWriteCloser) Close() error {
	return io.ErrShortWrite
}

func TestCreateScrap_HappyPath_Success(t *testing.T) {
	s := &test_gcsclient.GCSClient{}

	var w myWriteCloser

	s.On("FileWriter", testutils.AnyContext, fmt.Sprintf("scraps/svg/%s", svgHash),
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

func TestCreateScrap_InvalidScrapType_Failure(t *testing.T) {
	s := &test_gcsclient.GCSClient{}
	se := New(s)
	sentBody := ScrapBody{
		Type: Type("not-a-known-type"),
		Body: "<svg></svg>",
	}
	_, err := se.CreateScrap(context.Background(), sentBody)
	require.Error(t, err)
}

func TestCreateScrap_FileWriterFailsOnWrite_Failure(t *testing.T) {
	s := &test_gcsclient.GCSClient{}

	var w myErrorOnWriteWriteCloser

	s.On("FileWriter", testutils.AnyContext, fmt.Sprintf("scraps/svg/%s", svgHash),
		gcs.FileWriteOptions{
			ContentEncoding: "gzip",
			ContentType:     "application/json",
		}).Return(&w)

	se := New(s)
	sentBody := ScrapBody{
		Type: SVG,
		Body: "<svg></svg>",
	}
	_, err := se.CreateScrap(context.Background(), sentBody)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Failed to write JSON body.")
}

func TestCreateScrap_FileWriterFailsOnClose_Failure(t *testing.T) {
	s := &test_gcsclient.GCSClient{}

	var w myErrorOnCloseWriteCloser

	s.On("FileWriter", testutils.AnyContext, fmt.Sprintf("scraps/svg/%s", svgHash),
		gcs.FileWriteOptions{
			ContentEncoding: "gzip",
			ContentType:     "application/json",
		}).Return(&w)

	se := New(s)
	sentBody := ScrapBody{
		Type: SVG,
		Body: "<svg></svg>",
	}
	_, err := se.CreateScrap(context.Background(), sentBody)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Failed to close GCS Storage writer.")
}

func TestPutName_HappyPath_Success(t *testing.T) {
	s := &test_gcsclient.GCSClient{}

	var w myWriteCloser

	s.On("FileWriter", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName),
		gcs.FileWriteOptions{
			ContentType: "application/json",
		}).Return(&w)

	se := New(s)
	sentName := Name{
		Hash:        svgHash,
		Description: "Some description of the named scrap.",
	}
	err := se.PutName(context.Background(), SVG, scrapName, sentName)
	require.NoError(t, err)

	// Decode the written body to confirm it matches what we sent.
	r := bytes.NewReader(w.Bytes())
	var storedName Name
	err = json.NewDecoder(r).Decode(&storedName)
	require.NoError(t, err)
	require.Equal(t, storedName, sentName)
}
