package goldclient

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/gold-client/go/mocks"
)

func TestGetWithRetries_OneAttempt_Success(t *testing.T) {
	unittest.SmallTest(t)

	mh := &mocks.HTTPClient{}
	defer mh.AssertExpectations(t)

	url := "example.com"
	mh.On("Get", url).Return(httpResponse([]byte("Hello, world!"), "200 OK", http.StatusOK), nil).Once()

	b, err := getWithRetries(mh, url)
	assert.NoError(t, err)
	assert.Equal(t, []byte("Hello, world!"), b)
}

func TestGetWithRetries_MultipleAttempts_Success(t *testing.T) {
	unittest.LargeTest(t) // Function under test sleeps for several milliseconds before retries.

	mh := &mocks.HTTPClient{}
	defer mh.AssertExpectations(t)

	url := "example.com"
	mh.On("Get", url).Return(nil, errors.New("http.Client error")).Once()
	mh.On("Get", url).Return(httpResponse([]byte("Should be ignored."), "500 Internal Server Error", http.StatusInternalServerError), nil).Once()
	mh.On("Get", url).Return(httpResponse([]byte("Hello, world!"), "200 OK", http.StatusOK), nil).Once()

	b, err := getWithRetries(mh, url)
	assert.NoError(t, err)
	assert.Equal(t, []byte("Hello, world!"), b)
}

func TestGetWithRetries_MultipleAttempts_Error(t *testing.T) {
	unittest.LargeTest(t) // Function under test sleeps for several milliseconds before retries.

	mh := &mocks.HTTPClient{}
	defer mh.AssertExpectations(t)

	url := "example.com"
	mh.On("Get", url).Return(httpResponse([]byte("Should be ignored."), "404 Not found", http.StatusNotFound), nil).Times(5)

	_, err := getWithRetries(mh, url)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestPost_Success(t *testing.T) {
	unittest.SmallTest(t)

	mh := &mocks.HTTPClient{}
	defer mh.AssertExpectations(t)

	url := "example.com"
	contentType := "text/plain"
	body := strings.NewReader("Payload")

	mh.On("Post", url, contentType, body).Return(httpResponse([]byte("Hello, world!"), "200 OK", http.StatusOK), nil)

	b, err := post(mh, url, contentType, body)
	assert.NoError(t, err)
	assert.Equal(t, []byte("Hello, world!"), b)
}

func TestPost_HttpClientError_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	mh := &mocks.HTTPClient{}
	defer mh.AssertExpectations(t)

	url := "example.com"
	contentType := "text/plain"
	body := strings.NewReader("Payload")

	mh.On("Post", url, contentType, body).Return(nil, errors.New("http.Client error"))

	_, err := post(mh, url, contentType, body)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "http.Client error")
}

func TestPost_InternalServerError_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	mh := &mocks.HTTPClient{}
	defer mh.AssertExpectations(t)

	url := "example.com"
	contentType := "text/plain"
	body := strings.NewReader("Payload")

	mh.On("Post", url, contentType, body).Return(httpResponse([]byte("Should be ignored."), "500 Internal Server Error", http.StatusInternalServerError), nil)

	_, err := post(mh, url, contentType, body)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}
