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
type myReadWriteCloser struct {
	bytes.Buffer
}

func (*myReadWriteCloser) Close() error {
	return nil
}

// myErrorOnWriteWriteCloser turns a bytes.Buffer from an io.Writer to an io.WriteCloser that errors when Writing.
type myErrorOnWriteReadWriteCloser struct {
	bytes.Buffer
}

func (*myErrorOnWriteReadWriteCloser) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func (*myErrorOnWriteReadWriteCloser) Close() error {
	return nil
}

// myErrorOnWriteWriteCloser turns a bytes.Buffer from an io.Writer to an io.WriteCloser that errors when Closing.
type myErrorOnCloseReadWriteCloser struct {
	bytes.Buffer
}

func (*myErrorOnCloseReadWriteCloser) Close() error {
	return io.ErrShortWrite
}

func TestCreateScrap_HappyPath_Success(t *testing.T) {
	s := &test_gcsclient.GCSClient{}

	var w myReadWriteCloser

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

	var w myErrorOnWriteReadWriteCloser

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

	var w myErrorOnCloseReadWriteCloser

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

	var w myReadWriteCloser

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

func TestPutName_InvalidName_Failure(t *testing.T) {
	se := New(&test_gcsclient.GCSClient{})
	sentName := Name{
		Hash: svgHash,
	}
	err := se.PutName(context.Background(), SVG, "not-a-valid-scrap-name-no-@-prefix", sentName)
	require.Error(t, err)
	require.Contains(t, err.Error(), errInvalidScrapName.Error())
}

func TestPutName_InvalidHash_Failure(t *testing.T) {
	se := New(&test_gcsclient.GCSClient{})
	sentName := Name{
		Hash: "this is not a valid SHA256 hash",
	}
	err := se.PutName(context.Background(), SVG, scrapName, sentName)
	require.Error(t, err)
	require.Contains(t, err.Error(), errInvalidHash.Error())
}

func TestPutName_FileWriterWriteFails_Failure(t *testing.T) {
	s := &test_gcsclient.GCSClient{}

	var w myErrorOnWriteReadWriteCloser

	s.On("FileWriter", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName),
		gcs.FileWriteOptions{
			ContentType: "application/json",
		}).Return(&w)

	se := New(s)
	sentName := Name{
		Hash: svgHash,
	}
	err := se.PutName(context.Background(), SVG, scrapName, sentName)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Failed to encode JSON.")
}

func TestPutName_FileWriterCloseFails_Failure(t *testing.T) {
	s := &test_gcsclient.GCSClient{}

	var w myErrorOnCloseReadWriteCloser

	s.On("FileWriter", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName),
		gcs.FileWriteOptions{
			ContentType: "application/json",
		}).Return(&w)

	se := New(s)
	sentName := Name{
		Hash: svgHash,
	}
	err := se.PutName(context.Background(), SVG, scrapName, sentName)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Failed to close GCS Storage writer.")
}

func TestGettName_HappyPath_Success(t *testing.T) {
	s := &test_gcsclient.GCSClient{}

	var r myReadWriteCloser
	storedName := Name{
		Hash:        svgHash,
		Description: "Some description of the named scrap.",
	}
	err := json.NewEncoder(&r).Encode(storedName)
	require.NoError(t, err)

	s.On("FileReader", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName)).Return(&r, nil)

	se := New(s)
	retrievedName, err := se.GetName(context.Background(), SVG, scrapName)
	require.NoError(t, err)
	require.NoError(t, err)
	require.Equal(t, storedName, retrievedName)
}

func TestGettName_InvalidScrapName_ReturnsError(t *testing.T) {
	se := New(&test_gcsclient.GCSClient{})
	_, err := se.GetName(context.Background(), SVG, "not-a-valid-scrap-name-missing-@-prefix")
	require.Error(t, err)
	require.Contains(t, err.Error(), errInvalidScrapName.Error())
}
