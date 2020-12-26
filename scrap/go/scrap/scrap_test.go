// Package scrap defines the scrap types and functions on them.
package scrap

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/test_gcsclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

const svgHash = "975c5b1ad481d0e8b0875e03b0bf2b375da68593d8beac607f23f40b2e53ca31"

const scrapName = "@MyScrapName"

const scrapName2 = "@MyScrapName2"

const invalidScrapName = "not-a-valid-scrap-name-missing-@-prefix"

const invalidScrapType = "not-a-valid-scrap-type"

var errMyMockError = errors.New("my error returned from mock GCSClient")

// myWriteCloser is a wrapper that turns a bytes.Buffer from an io.Writer to an io.WriteCloser.
type myReadWriteCloser struct {
	bytes.Buffer
}

func (*myReadWriteCloser) Close() error {
	return nil
}

// myErrorOnWriteWriteCloser turns a bytes.Buffer from an io.Writer to an io.WriteCloser that errors when Writing.
type myErrorReadWriteCloser struct {
	bytes.Buffer
}

func (*myErrorReadWriteCloser) Read([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func (*myErrorReadWriteCloser) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func (*myErrorReadWriteCloser) Close() error {
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
	unittest.SmallTest(t)
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
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}
	se := New(s)
	sentBody := ScrapBody{
		Type: Type("not-a-known-type"),
		Body: "<svg></svg>",
	}
	_, err := se.CreateScrap(context.Background(), sentBody)
	require.Contains(t, err.Error(), errInvalidScrapType.Error())
}

func TestCreateScrap_FileWriterFailsOnWrite_Failure(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	var w myErrorReadWriteCloser

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
	require.Contains(t, err.Error(), "Failed to write JSON body.")
}

func TestCreateScrap_FileWriterFailsOnClose_Failure(t *testing.T) {
	unittest.SmallTest(t)
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
	require.Contains(t, err.Error(), "Failed to close GCS Storage writer.")
}

func TestPutName_HappyPath_Success(t *testing.T) {
	unittest.SmallTest(t)
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
	unittest.SmallTest(t)
	se := New(&test_gcsclient.GCSClient{})
	sentName := Name{
		Hash: svgHash,
	}
	err := se.PutName(context.Background(), SVG, invalidScrapName, sentName)
	require.Contains(t, err.Error(), errInvalidScrapName.Error())
}

func TestPutName_InvalidType_Failure(t *testing.T) {
	unittest.SmallTest(t)
	se := New(&test_gcsclient.GCSClient{})
	sentName := Name{
		Hash: svgHash,
	}
	err := se.PutName(context.Background(), invalidScrapType, scrapName, sentName)
	require.Contains(t, err.Error(), errInvalidScrapType.Error())
}

func TestPutName_InvalidHash_Failure(t *testing.T) {
	unittest.SmallTest(t)
	se := New(&test_gcsclient.GCSClient{})
	sentName := Name{
		Hash: "this is not a valid SHA256 hash",
	}
	err := se.PutName(context.Background(), SVG, scrapName, sentName)
	require.Contains(t, err.Error(), errInvalidHash.Error())
}

func TestPutName_FileWriterWriteFails_Failure(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	var w myErrorReadWriteCloser

	s.On("FileWriter", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName),
		gcs.FileWriteOptions{
			ContentType: "application/json",
		}).Return(&w)

	se := New(s)
	sentName := Name{
		Hash: svgHash,
	}
	err := se.PutName(context.Background(), SVG, scrapName, sentName)
	require.Contains(t, err.Error(), "Failed to encode JSON.")
}

func TestPutName_FileWriterCloseFails_Failure(t *testing.T) {
	unittest.SmallTest(t)
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
	require.Contains(t, err.Error(), "Failed to close GCS Storage writer.")
}

func TestGetName_HappyPath_Success(t *testing.T) {
	unittest.SmallTest(t)
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
	require.Equal(t, storedName, retrievedName)
}

func TestGetName_FileReaderReadFails_Failure(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	var r myErrorReadWriteCloser
	s.On("FileReader", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName)).Return(&r, nil)

	se := New(s)
	_, err := se.GetName(context.Background(), SVG, scrapName)
	require.Contains(t, err.Error(), "Failed to decode body.")
}

func TestGetName_InvalidScrapName_Failure(t *testing.T) {
	unittest.SmallTest(t)
	se := New(&test_gcsclient.GCSClient{})
	_, err := se.GetName(context.Background(), SVG, invalidScrapName)
	require.Contains(t, err.Error(), errInvalidScrapName.Error())
}

func TestGetName_InvalidScrapType_Failure(t *testing.T) {
	unittest.SmallTest(t)
	se := New(&test_gcsclient.GCSClient{})
	_, err := se.GetName(context.Background(), invalidScrapType, scrapName)
	require.Contains(t, err.Error(), errInvalidScrapType.Error())
}

func TestGetName_FileReaderFails_Failure(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}
	s.On("FileReader", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName)).Return(nil, errMyMockError)
	se := New(s)
	_, err := se.GetName(context.Background(), SVG, scrapName)
	require.Contains(t, err.Error(), "Failed to open name.")
}

func TestDeleteName_HappyPath_Success(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	s.On("DeleteFile", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName)).Return(nil)

	se := New(s)
	err := se.DeleteName(context.Background(), SVG, scrapName)
	require.NoError(t, err)
}

func TestDeleteName_DeleteFileFails_Failure(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	s.On("DeleteFile", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName)).Return(errMyMockError)

	se := New(s)
	err := se.DeleteName(context.Background(), SVG, scrapName)
	require.Contains(t, err.Error(), "Failed to delete name.")
}

func TestDeleteName_InvalidType_Failure(t *testing.T) {
	unittest.SmallTest(t)
	se := New(&test_gcsclient.GCSClient{})
	err := se.DeleteName(context.Background(), invalidScrapType, scrapName)
	require.Contains(t, err.Error(), errInvalidScrapType.Error())
}

func TestDeleteName_InvalidName_Failure(t *testing.T) {
	unittest.SmallTest(t)
	se := New(&test_gcsclient.GCSClient{})
	err := se.DeleteName(context.Background(), SVG, invalidScrapName)
	require.Contains(t, err.Error(), errInvalidScrapName.Error())
}

func TestListNames_HappyPath_Success(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	s.On("AllFilesInDirectory", testutils.AnyContext, fmt.Sprintf("names/svg/"), mock.Anything).Run(func(args mock.Arguments) {
		cb := args[2].(func(item *storage.ObjectAttrs))
		cb(&storage.ObjectAttrs{Name: scrapName})
		cb(&storage.ObjectAttrs{Name: scrapName2})
	}).Return(nil)

	se := New(s)
	list, err := se.ListNames(context.Background(), SVG)
	require.NoError(t, err)
	require.Equal(t, []string{scrapName, scrapName2}, list)
}

func TestListNames_AllFilesInDirectoryFailure_Failure(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	s.On("AllFilesInDirectory", testutils.AnyContext, fmt.Sprintf("names/svg/"), mock.Anything).Return(errMyMockError)

	se := New(s)
	_, err := se.ListNames(context.Background(), SVG)
	require.Contains(t, err.Error(), "Failed to read directory.")
}

func TestListNames_InvalidType_Failure(t *testing.T) {
	unittest.SmallTest(t)
	se := New(&test_gcsclient.GCSClient{})
	_, err := se.ListNames(context.Background(), invalidScrapType)
	require.Contains(t, err.Error(), errInvalidScrapType.Error())
}
