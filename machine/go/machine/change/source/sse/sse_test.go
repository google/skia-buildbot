package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/r3labs/sse/v2"
	"github.com/stretchr/testify/require"
)

const (
	machineID = "skia-rpi2-rack4-shelf2-001"
)

func TestSSENewFromClient_HappyPath(t *testing.T) {
	ctx := context.Background()

	// Create new SSE server and attach to http server.
	sseServer := sse.New()
	_ = sseServer.CreateStream(machineID)
	httpServer := httptest.NewServer(sseServer)

	// Create our client, which talks to the above server.
	sourceSSE, err := NewFromClient(ctx, httpServer.Client(), httpServer.URL, machineID)
	require.NoError(t, err)

	// Send a message that the client should be registered to receive.
	sseServer.Publish(machineID, &sse.Event{
		Data: []byte("foo"),
	})

	// The test will timeout if the event isn't received.
	<-sourceSSE.Start(ctx)
}

func TestSSENewFromClient_BadServerURL_ReturnsError(t *testing.T) {
	_, err := NewFromClient(context.Background(), http.DefaultClient, "::: this is not a valid URL", machineID)
	require.Error(t, err)
}
