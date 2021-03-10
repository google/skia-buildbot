package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/test_gcsclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestUploadHandler_ValidJSONFile_Success(t *testing.T) {
	unittest.SmallTest(t)

	// This hash is derived from the contents of the input file.
	const expectedUploadName = "c54ac366e2c358cab4e6431cb47d6178/lottie.json"

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/does/not/matter", testutils.GetReader(t, "skottie-luma-matte-request.json"))

	mgc := test_gcsclient.NewMockClient()

	optsMatcher := mock.MatchedBy(func(opts gcs.FileWriteOptions) bool {
		assert.Equal(t, jsonContentType, opts.ContentEncoding)
		return true
	})
	jsonBuffer := &bytes.Buffer{}
	mgc.On("FileWriter", testutils.AnyContext, expectedUploadName, optsMatcher).Return(nopCloser(jsonBuffer), nil)

	srv := Server{
		gcsClient: mgc,
	}
	srv.uploadHandler(w, r)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, jsonContentType, resp.Header.Get(contentTypeHeader))
	respBody, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	// spot check the return body
	assert.Contains(t, string(respBody), `"hash":"c54ac366e2c358cab4e6431cb47d6178"`)
	assert.Len(t, respBody, 3229)
	// spot check the written data to GCS
	writtenJSON := jsonBuffer.String()
	assert.Contains(t, writtenJSON, `"filename":"luma-matte.json"`)
	assert.Len(t, writtenJSON, 3250)
}

const (
	contentTypeHeader = "Content-Type"
	jsonContentType   = "application/json"
)

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }

func nopCloser(w io.Writer) io.WriteCloser {
	return nopWriteCloser{w}
}
