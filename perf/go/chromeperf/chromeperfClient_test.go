package chromeperf

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendRegression_RequestIsValid_Success(t *testing.T) {
	anomalyResponse := &ChromePerfResponse{
		AnomalyId:    "1234",
		AlertGroupId: "5678",
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewEncoder(w).Encode(anomalyResponse)
		require.NoError(t, err)
	}))

	ctx := context.Background()
	cpClient, err := NewChromePerfClient(context.Background(), ts.URL)
	assert.Nil(t, err, "No error expected when creating a new client.")
	response, err := cpClient.SendRegression(ctx, "/some/path", 1, 10, "proj", false, "bot", false, 5, 10)
	assert.NotNil(t, response)
	assert.Nil(t, err, "No error expected in the SendRegression call.")
	assert.Equal(t, anomalyResponse, response)
}

func TestSendRegression_ServerReturnsError_ReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))

	defer ts.Close()
	ctx := context.Background()
	cpClient, err := NewChromePerfClient(context.Background(), ts.URL)
	assert.Nil(t, err, "No error expected when creating a new client")
	response, err := cpClient.SendRegression(ctx, "/some/path", 1, 10, "proj", false, "bot", false, 5, 10)
	assert.Nil(t, response, "Nil response expected for server error.")
	assert.NotNil(t, err, "Non nil error expected.")
}
