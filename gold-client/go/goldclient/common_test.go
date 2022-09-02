package goldclient

import (
	"context"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/gold-client/go/mocks"
)

func TestGetWithRetries_OneAttempt_Success(t *testing.T) {

	mh := &mocks.HTTPClient{}
	defer mh.AssertExpectations(t)

	url := "example.com"
	mh.On("Get", url).Return(httpResponse("Hello, world!", "200 OK", http.StatusOK), nil).Once()

	ctx := WithContext(context.Background(), nil, mh, nil)
	b, err := getWithRetries(ctx, url)
	assert.NoError(t, err)
	assert.Equal(t, []byte("Hello, world!"), b)
}

func TestGetWithRetries_MultipleAttempts_Success(t *testing.T) {
	mh := &mocks.HTTPClient{}
	defer mh.AssertExpectations(t)

	url := "example.com"
	mh.On("Get", url).Return(nil, errors.New("http.Client error")).Once()
	mh.On("Get", url).Return(httpResponse("Should be ignored.", "500 Internal Server Error", http.StatusInternalServerError), nil).Once()
	mh.On("Get", url).Return(httpResponse("Hello, world!", "200 OK", http.StatusOK), nil).Once()

	ctx := WithContext(context.Background(), nil, mh, nil)
	b, err := getWithRetries(ctx, url)
	assert.NoError(t, err)
	assert.Equal(t, []byte("Hello, world!"), b)
}

func TestGetWithRetries_MultipleAttempts_Error(t *testing.T) {
	mh := &mocks.HTTPClient{}
	defer mh.AssertExpectations(t)

	url := "example.com"
	mh.On("Get", url).Return(httpResponse("Should be ignored.", "404 Not found", http.StatusNotFound), nil)

	ctx := WithContext(context.Background(), nil, mh, nil)
	_, err := getWithRetries(ctx, url)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestPost_Success(t *testing.T) {

	mh := &mocks.HTTPClient{}
	defer mh.AssertExpectations(t)

	url := "example.com"
	contentType := "text/plain"
	body := strings.NewReader("Payload")

	mh.On("Post", url, contentType, body).Return(httpResponse("Hello, world!", "200 OK", http.StatusOK), nil)

	ctx := WithContext(context.Background(), nil, mh, nil)
	b, err := post(ctx, url, contentType, body)
	assert.NoError(t, err)
	assert.Equal(t, []byte("Hello, world!"), b)
}

func TestPost_HttpClientError_ReturnsError(t *testing.T) {

	mh := &mocks.HTTPClient{}
	defer mh.AssertExpectations(t)

	url := "example.com"
	contentType := "text/plain"
	body := strings.NewReader("Payload")

	mh.On("Post", url, contentType, body).Return(nil, errors.New("http.Client error"))

	ctx := WithContext(context.Background(), nil, mh, nil)
	_, err := post(ctx, url, contentType, body)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "http.Client error")
}

func TestPost_InternalServerError_ReturnsError(t *testing.T) {

	mh := &mocks.HTTPClient{}
	defer mh.AssertExpectations(t)

	url := "example.com"
	contentType := "text/plain"
	body := strings.NewReader("Payload")

	mh.On("Post", url, contentType, body).Return(httpResponse("Should be ignored.", "500 Internal Server Error", http.StatusInternalServerError), nil)

	ctx := WithContext(context.Background(), nil, mh, nil)
	_, err := post(ctx, url, contentType, body)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func httpResponse(body, status string, statusCode int) *http.Response {
	return &http.Response{
		Body:       ioutil.NopCloser(strings.NewReader(body)),
		Status:     status,
		StatusCode: statusCode,
	}
}
