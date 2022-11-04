package sser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/r3labs/sse/v2"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sser/mocks"
	"go.skia.org/infra/go/testutils"
)

const (
	streamName = "testStreamName"
	eventValue = "this is a test message"
)

func TestServerImplPodIPToURL_HappyPath(t *testing.T) {
	s := &ServerImpl{
		internalPort: 4000,
	}
	require.Equal(t, "http://192.168.1.1:4000/api/json/v1/send", s.podIPToURL("192.168.1.1"))
}

func createServerAndFrontendForTest(t *testing.T) (context.Context, *ServerImpl, *httptest.Server) {
	ctx := context.Background()

	// Create a PeerFinder that just returns localhost.
	peerFinderMock := mocks.NewPeerFinder(t)
	ipCh := make(chan []string)
	var castChan <-chan []string = ipCh
	peerFinderMock.On("Start", testutils.AnyContext).Return([]string{"127.0.0.1"}, castChan, nil)

	// Create a new Server that uses any available port to listen for peer connections.
	sserServer, err := New(0, peerFinderMock)
	require.NoError(t, err)
	err = sserServer.Start(ctx)
	require.NoError(t, err)

	// Create a new web server, aka the frontend, at a different random port,
	// that handles incoming SSE client connections.
	frontend := httptest.NewServer(sserServer.ClientConnectionHandler(ctx))
	t.Cleanup(frontend.Close)

	metrics2.GetCounter(clientConnectionsMetricName, map[string]string{QueryParameterName: streamName}).Reset()

	return ctx, sserServer, frontend
}

func TestServer_HappyPath(t *testing.T) {
	ctx, sserServer, frontend := createServerAndFrontendForTest(t)

	// Create an SSE client that talks to the above frontend.
	client := sse.NewClient(frontend.URL + PeerEndpointURLPath)

	// Listen for events on the given channel.
	events := make(chan *sse.Event)
	err := client.SubscribeChan(streamName, events)
	t.Cleanup(func() {
		client.Unsubscribe(events)
	})
	require.NoError(t, err)

	// Send an event via the Server, which the client should receive via the frontend.
	sserServer.Send(ctx, streamName, eventValue)

	// Confirm the client received the correct event.
	e := <-events
	require.Equal(t, eventValue, string(e.Data))

	require.Equal(t, int64(1),
		metrics2.GetCounter(clientConnectionsMetricName, map[string]string{QueryParameterName: streamName}).Get())
}

func TestServer_PeerFinderReturnsError_StartReturnsError(t *testing.T) {
	ctx := context.Background()

	// Create a PeerFinder that returns an error.
	peerFinderMock := mocks.NewPeerFinder(t)
	peerFinderMock.On("Start", testutils.AnyContext).Return(nil, nil, myMockErr)

	// Create a new Server that uses any available port to listen for peer connections.
	s, err := New(0, peerFinderMock)
	require.NoError(t, err)
	err = s.Start(ctx)
	require.Error(t, err)
}

func TestServer_TwoClientsForSameStream_BothReceiveEvents(t *testing.T) {
	ctx, sserServer, frontend := createServerAndFrontendForTest(t)

	// Create an SSE client that talks to the above frontend.
	client1 := sse.NewClient(frontend.URL + PeerEndpointURLPath)
	events1 := make(chan *sse.Event)
	err := client1.SubscribeChan(streamName, events1)
	t.Cleanup(func() {
		client1.Unsubscribe(events1)
	})
	require.NoError(t, err)

	client2 := sse.NewClient(frontend.URL + PeerEndpointURLPath)
	events2 := make(chan *sse.Event)
	err = client2.SubscribeChan(streamName, events2)
	t.Cleanup(func() {
		client2.Unsubscribe(events2)
	})
	require.NoError(t, err)

	// Send an event via the Server, which the client should receive via the frontend.
	sserServer.Send(ctx, streamName, eventValue)

	// Confirm the client received the correct event.
	e := <-events1
	require.Equal(t, eventValue, string(e.Data))

	e = <-events2
	require.Equal(t, eventValue, string(e.Data))

	require.Equal(t, int64(2),
		metrics2.GetCounter(clientConnectionsMetricName, map[string]string{QueryParameterName: streamName}).Get())
}

func TestClientConnectionHandler_NoStreamNameProvided_ReturnsStatusBadRequest(t *testing.T) {
	ctx, sserServer, _ := createServerAndFrontendForTest(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/just/a/query/path/with/no/query/parameters", nil)

	sserServer.ClientConnectionHandler(ctx)(w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
}
