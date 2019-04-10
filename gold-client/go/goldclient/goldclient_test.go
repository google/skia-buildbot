package goldclient

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/gold-client/go/mocks"
	"go.skia.org/infra/golden/go/jsonio"

	assert "github.com/stretchr/testify/require"
)

const (
	INSTANCE_ID_TEST     = "testing"
	ISSUE_ID_TEST        = int64(867)
	PATCHSET_ID_TEST     = int64(5309)
	BUILD_BUCKET_ID_TEST = int64(117)
	IMG_PATH_TEST        = "/path/to/images/fake.png"
)

// Report an
func TestReport(t *testing.T) {
	testutils.SmallTest(t)

	wd, cleanup := testutils.TempDir(t)
	//fmt.Printf("workdir %#v\n", wd)
	defer cleanup()

	config := &GoldClientConfig{
		InstanceID:   INSTANCE_ID_TEST,
		WorkDir:      wd,
		PassFailStep: false,
	}

	gr := &jsonio.GoldResults{
		GitHash: "abcd1234",
		Key: map[string]string{
			"os":  "WinTest",
			"gpu": "GPUTest",
		},
		Issue:         ISSUE_ID_TEST,
		Patchset:      PATCHSET_ID_TEST,
		BuildBucketID: BUILD_BUCKET_ID_TEST,
	}

	imgData := []byte("some bytes")
	imgHash := "9d0568469d206c1aedf1b71f12f474bc"

	auth, httpClient, uploader := makeMocks()
	defer auth.AssertExpectations(t)
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse([]byte("none"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	expectations := httpResponse([]byte("{}"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234?issue=867").Return(expectations, nil)

	expectedUploadPath := "gs://skia-gold-testing/dm-images-v1/" + imgHash + ".png"
	uploader.On("UploadBytes", imgData, IMG_PATH_TEST, expectedUploadPath).Return(nil)

	goldClient, err := NewCloudClient(auth, config, gr)
	assert.NoError(t, err)

	goldClient.TESTING_overrideLoadAndHashImage(func(path string) ([]byte, string, error) {
		assert.Equal(t, IMG_PATH_TEST, path)
		return imgData, imgHash, nil
	})

	pass, err := goldClient.Test("first-test", IMG_PATH_TEST)
	assert.NoError(t, err)
	assert.True(t, pass)
}

func makeMocks() (*MockAuthOpt, *mocks.HTTPClient, *mocks.GoldUploader) {
	mh := mocks.HTTPClient{}
	mg := mocks.GoldUploader{}
	ma := MockAuthOpt{}
	ma.On("Validate").Return(nil)
	ma.On("GetHTTPClient").Return(&mh, nil)
	ma.On("GetGoldUploader").Return(&mg, nil)
	return &ma, &mh, &mg
}

func httpResponse(body []byte, status string, statusCode int) *http.Response {
	return &http.Response{
		Body:       &respBodyCloser{bytes.NewReader(body)},
		Status:     status,
		StatusCode: statusCode,
	}
}

// respBodyCloser is a wrapper which lets us pretend to implement io.ReadCloser
// by wrapping a bytes.Reader.
type respBodyCloser struct {
	io.Reader
}

// Close is a stub method which lets us pretend to implement io.ReadCloser.
func (r respBodyCloser) Close() error {
	return nil
}
