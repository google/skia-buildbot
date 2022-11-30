// Package httpsink sends event to the machine server via HTTP.
package httpsink

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/machine/go/machine"
)

func TestNewFromClient_InvalidURL_ReturnsError(t *testing.T) {
	_, err := NewFromClient(nil, "this is not a valid url\n")
	require.Contains(t, err.Error(), "parsing serverURL")
}

func TestNewFromClient_HappyPath(t *testing.T) {
	actual, err := NewFromClient(http.DefaultClient, "https://example.org:8080")
	require.NoError(t, err)
	require.Equal(t, "https://example.org:8080/json/v1/machine/event/", actual.targetURL)
	require.Equal(t, http.DefaultClient, actual.client)
}

func TestSend_HappyPath(t *testing.T) {
	var actual machine.Event
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&actual)
		require.NoError(t, err)
	}))
	sink, err := NewFromClient(s.Client(), s.URL)
	require.NoError(t, err)
	event := machine.NewEvent()
	event.Host = machine.Host{
		Name: "skia-rpi2-rack4-shelf1-020",
	}
	err = sink.Send(context.Background(), event)
	require.NoError(t, err)
	assertdeep.Equal(t, event, actual)
}

func TestSend_HTTPRequestFails_ReturnsError(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "failed", http.StatusInternalServerError)
	}))
	sink, err := NewFromClient(s.Client(), s.URL)
	require.NoError(t, err)
	event := machine.NewEvent()
	event.Host = machine.Host{
		Name: "skia-rpi2-rack4-shelf1-020",
	}
	err = sink.Send(context.Background(), event)
	require.Contains(t, err.Error(), "non-200 status code")
}
