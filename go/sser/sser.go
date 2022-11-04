// Package sser allows handling Server-Sent Events (SSE) connections from across
// kubernetes Pod replicas, for example a Deployment or a StatefulSet, and also
// allows sending events from any of those Pods to all clients listening on a
// stream regardless of which Pod they are connected to.
//
// The diagram below assumes the internal port is :7000, and the public
// facing/frontend port is :8000. These are only for example and both are
// changeable. In the diagram there are 3 pods in a Deployment and each pod is
// able to handle incoming connections from a client wanting to receive a
// Server-Sent Event. The clients always connect to the frontend. (1)(2)
//
// At any point, due to internal processing, the app can send a message to every
// client listening on the "foo" stream by callling Server.Send(ctx, "foo", "my
// message"). The sser.Server then sends the stream name and message to every
// pod via HTTP POST to every pod's internal endpoint. (4)
//
// When each sser.Server handles that request it the sends the message to every
// client connnected to that Pod that is listening on that stream. (5)
//
//
//                             +---------------------------+
//                             |                           |
//                             | Pod 1                     |
//                             |                           |
//                         +-----> :7000/api/json/v1/send  |
//                         |   |                           |  (1) Client A - listen on "foo"
//                         |   |   :8000/events     <-------------------------------------------
//                         |   |                           |  (5) Client A receives "my message"
//                         |   +---------------------------+
//                         |
//                         |   +---------------------------+
//                         |   |                           |
//                    (4)  +---| Pod 2                     |
//     HTTP Post to every  |   |                           |  (3) Send(ctx, "foo", "my message")
//     peer Pods internal  +-----> :7000/api/json/v1/send  |
//     port.               |   |                           |
//                         |   |   :8000/events            |
//                         |   |                           |
//                         |   +---------------------------+
//                         |
//                         |   +---------------------------+
//                         |   |                           |
//                         |   | Pod N                     |
//                         |   |                           |
//                         +-----> :7000/api/json/v1/send  |
//                             |                           |  (2) Client B - listen on "foo"
//                             |   :8000/events    <--------------------------------------------
//                             |                           |  (5) Client B receives "my message"
//                             +---------------------------+
//
// Finding all the peer Pods is handled via the Kubernetes API, and any changes
// to the peers are handled through a watch, so the list of peers is always up
// to date.
//
// https://kubernetes.io/docs/reference/using-api/api-concepts/#efficient-detection-of-changes
//
// This package makes no delivery guarantees and has all the same race
// conditions that are possible with the underlying SSE protocol.
//
// The events passed around to the peers are all on internal ports, so we don't
// have to protect those ports, unlike the frontend ports.
package sser

import (
	"context"
	"net/http"
)

const (
	// PeerEndpointURLPath is the URL path that the server will listen on for
	// incoming events that need to be distributed to all connected clients.
	PeerEndpointURLPath = "/api/json/v1/send"

	// QueryParameterName is the query parameter that the sse client and server library look
	// for by default.
	QueryParameterName = "stream"
)

// PeerFinder finds the IP addresses of all of the pods that make up this set of
// replicas.
type PeerFinder interface {
	// Start returns a slice of IP Addresses that represent all the peers of
	// this pod that are in the Running state. It also returns a channel that
	// will provide updated slices of IP addresses every time they change. Note
	// that this includes the running pod itself.
	Start(ctx context.Context) ([]string, <-chan []string, error)
}

// Server allows handling incoming SSE connection requests, and sending events
// across multiple pods.
type Server interface {
	// Start the SSE server.
	//
	// Start will automatically start listening on an internal port
	// at PeerEndpointURLPath.
	//
	// Start must be called before ClientConnectionHandler() or Send().
	Start(ctx context.Context) error

	// ClientConnectionHandler returns an http.Handler that can be registered to
	// handle requests from SSE clients on the frontend port.
	ClientConnectionHandler(ctx context.Context) http.HandlerFunc

	// Send a message to all connections on all peer pods that are listening for
	// events from the given stream name.
	Send(ctx context.Context, stream string, msg string)
}
