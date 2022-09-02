package main

import (
	"bytes"
	"image"
	"image/png"
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
)

func TestUploadHandler_ValidJSONFile_Success(t *testing.T) {

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

	mgc.AssertExpectations(t)
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

// The AssetsZip in this test file is a base64 encoded zip file containing 6 small images copied
// from the skia repo's //resources/images folder. One of these is a 3x3 png image, which we
// make sure is uploaded as a valid PNG file.
func TestUploadHandler_ValidJSONWithAssetZip_FilesInZipUploaded(t *testing.T) {

	// This hash is derived from the contents of the input file.
	const expectedJSONName = "5f0e05cf5594b23cc98a1a31693a377c/lottie.json"

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/does/not/matter", testutils.GetReader(t, "skottie-images-asset.json"))

	mgc := test_gcsclient.NewMockClient()

	jsonOptsMatcher := mock.MatchedBy(func(opts gcs.FileWriteOptions) bool {
		return opts.ContentEncoding == jsonContentType
	})
	jsonBuffer := &bytes.Buffer{}
	mgc.On("FileWriter", testutils.AnyContext, expectedJSONName, jsonOptsMatcher).Return(nopCloser(jsonBuffer), nil)

	assetOptsMatcher := mock.MatchedBy(func(opts gcs.FileWriteOptions) bool {
		return opts.ContentEncoding == octetContentType
	})
	samplePNGBuffer := &bytes.Buffer{}
	mgc.On("FileWriter", testutils.AnyContext, "5f0e05cf5594b23cc98a1a31693a377c/assets/3x3.png", assetOptsMatcher).Return(nopCloser(samplePNGBuffer), nil)
	// We'll spot check just one of the images for content. The rest we can just make sure
	// the file names were created correctly. These file names match up with the names of
	// the files that were zipped up.
	mgc.On("FileWriter", testutils.AnyContext, "5f0e05cf5594b23cc98a1a31693a377c/assets/image_0.png", assetOptsMatcher).Return(newNopWriter(), nil)
	mgc.On("FileWriter", testutils.AnyContext, "5f0e05cf5594b23cc98a1a31693a377c/assets/image_1.png", assetOptsMatcher).Return(newNopWriter(), nil)
	mgc.On("FileWriter", testutils.AnyContext, "5f0e05cf5594b23cc98a1a31693a377c/assets/image_2.png", assetOptsMatcher).Return(newNopWriter(), nil)
	mgc.On("FileWriter", testutils.AnyContext, "5f0e05cf5594b23cc98a1a31693a377c/assets/image_3.png", assetOptsMatcher).Return(newNopWriter(), nil)
	mgc.On("FileWriter", testutils.AnyContext, "5f0e05cf5594b23cc98a1a31693a377c/assets/image_4.png", assetOptsMatcher).Return(newNopWriter(), nil)

	srv := Server{
		canUploadZips: true,
		gcsClient:     mgc,
	}
	srv.uploadHandler(w, r)

	mgc.AssertExpectations(t)
	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, jsonContentType, resp.Header.Get(contentTypeHeader))
	respBody, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	// spot check the return body
	assert.Contains(t, string(respBody), `"hash":"5f0e05cf5594b23cc98a1a31693a377c"`)
	assert.Len(t, respBody, 604)

	writtenJSON := jsonBuffer.String()
	assert.Contains(t, writtenJSON, `"filename":"skottie-image-asset.json"`)
	assert.Contains(t, writtenJSON, `"assetsZip":""`)
	assert.Contains(t, writtenJSON, `"assetsFilename":""`)
	assert.Len(t, writtenJSON, 634)

	// Make sure we upload a valid PNG file of size 3x3.
	pngBytes := samplePNGBuffer.Bytes()
	img, err := png.Decode(bytes.NewReader(pngBytes))
	require.NoError(t, err)
	assert.Equal(t, image.Rect(0, 0, 3, 3), img.Bounds())
}

const (
	contentTypeHeader = "Content-Type"
	jsonContentType   = "application/json"
	octetContentType  = "application/octet-stream"
)

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }

func nopCloser(w io.Writer) io.WriteCloser {
	return nopWriteCloser{w}
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
func newNopWriter() io.WriteCloser {
	return nopCloser(nopWriter{})
}
