package sse

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/r3labs/sse/v2"
	"github.com/stretchr/testify/require"
)

const (
	machineID = "skia-rpi2-rack4-shelf2-001"
)

func TestSSE_Send(t *testing.T) {
	ctx := context.Background()
	s, err := New(ctx, true, "ignored when local=true", "ignored when local=true", 0)
	require.NoError(t, err)

	// Stand up local HTTP server for the Server-Sent Event client to talk to.
	httpServer := httptest.NewServer(s.GetHandler(ctx))
	t.Cleanup(httpServer.Close)

	// Create SSE client and subscribe to update events for machineID.
	sseClient := sse.NewClient(httpServer.URL)
	events := make(chan *sse.Event)
	err = sseClient.SubscribeChan(machineID, events)
	t.Cleanup(func() {
		sseClient.Unsubscribe(events)
	})

	require.NoError(t, err)

	// Send an update message.
	err = s.Send(ctx, machineID)
	require.NoError(t, err)

	// Confirm message was received.
	_, ok := <-events
	require.True(t, ok)
}
