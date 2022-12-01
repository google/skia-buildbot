package sser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	sse "github.com/r3labs/sse/v2"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util_generics"
)

const (
	// 100 was picked as a rough guess.
	serverSendChannelSize = 100

	clientConnectionsMetricName = "sser_server_client_connections"
)

var (
	ErrStreamNameRequired = errors.New("a stream name is required as part of the query parameters")

	// ErrOnlySendNoneEmptyMessages because if you send an empty string, the client may mistake that as being no message.
	ErrOnlySendNoneEmptyMessages = errors.New("you cannot send the empty string as a message over SSE")
)

// Event is serialized as JSON to be sent from a server to each peer.
type Event struct {
	Stream string `json:"stream"`
	Msg    string `json:"msg"`
}

// ServerImpl implements Server.
type ServerImpl struct {
	// The HTTP port used for peer connections between all replicas of an app
	// running in kubernetes.
	internalPort int

	// Keeps the Server updated with all the peers.
	peerFinder PeerFinder

	// The SSE server implementation.
	server *sse.Server

	// Carries messages to be sent from Send() into the go routine that runs
	// from Start.
	sendCh chan Event

	// The current list of peer Pods that are in the Running state.
	peers map[string]*http.Client
}

// New returns a new Server.
func New(internalPort int, peerFinder PeerFinder) (*ServerImpl, error) {
	return &ServerImpl{
		internalPort: internalPort,
		peerFinder:   peerFinder,
		server:       sse.New(),
		sendCh:       make(chan Event, 100),
		peers:        map[string]*http.Client{},
	}, nil
}

func (s *ServerImpl) podIPToURL(ip string) string {
	var ret url.URL
	ret.Host = fmt.Sprintf("%s:%d", ip, s.internalPort)
	ret.Path = PeerEndpointURLPath
	ret.Scheme = "http"
	return ret.String()
}

func (s *ServerImpl) setPeersFromIPAddressSlice(ips []string) {
	newPeers := map[string]*http.Client{}
	for _, ip := range ips {
		u := s.podIPToURL(ip)
		newPeers[u] = util_generics.Get(s.peers, u, httputils.NewFastTimeoutClient())
	}
	s.peers = newPeers
}

func (s *ServerImpl) handlePeerNotification(w http.ResponseWriter, r *http.Request) {
	var e Event
	err := json.NewDecoder(r.Body).Decode(&e)
	if err != nil {
		httputils.ReportError(w, err, "invalid JSON", http.StatusBadRequest)
		return
	}

	s.server.Publish(e.Stream, &sse.Event{
		Data: []byte(e.Msg),
	})
}

// Start implements Server.
func (s *ServerImpl) Start(ctx context.Context) error {
	r := mux.NewRouter()
	r.HandleFunc(PeerEndpointURLPath, s.handlePeerNotification)

	// For testing purposes a 0 is allowed for internalPort, which will
	// select an available port on the machine.
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.internalPort))
	if err != nil {
		return skerr.Wrapf(err, "listening on port %d", s.internalPort)
	}

	// Since internalPort might have been 0, we set s.internalPort to the
	// Port that was selected.
	s.internalPort = listener.Addr().(*net.TCPAddr).Port

	// Start an HTTP server on internalPort to listen for events from peer pods.
	go func() {
		sklog.Fatal(http.Serve(listener, r))
	}()

	initial, ch, err := s.peerFinder.Start(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	s.setPeersFromIPAddressSlice(initial)

	// Start a Go routine that orchestrates both updates from PeerFinder, and
	// requests to send messages to all the peer pods. Avoid the need for a
	// mutex to protect s.peer by using channels and select.
	go func() {
		for {
			select {
			case newPeers := <-ch:
				s.setPeersFromIPAddressSlice(newPeers)
			case msg := <-s.sendCh:
				// Serialize msg into JSON.
				b, err := json.Marshal(msg)
				if err != nil {
					sklog.Errorf("failed to serialize Event: %s", err)
					continue
				}
				r := bytes.NewReader(b)
				// Send msg to each internal Peer endpoint.
				for peerURL, client := range s.peers {
					resp, err := client.Post(peerURL, "application/json", r)
					if err != nil {
						sklog.Errorf("notifying peer: %s", err)
						continue
					}
					_, err = r.Seek(0, io.SeekStart)
					if err != nil {
						sklog.Error("seeking to start of buffer: %s", err)
					}
					if resp.StatusCode >= 300 {
						sklog.Errorf("HTTP StatusCode Not OK: %s", resp.Status)
						continue
					}
				}
			}
		}
	}()

	return nil
}

// ClientConnectionHandler implements Server.
func (s *ServerImpl) ClientConnectionHandler(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		streamName := r.FormValue(QueryParameterName)
		if streamName == "" {
			httputils.ReportError(w, ErrStreamNameRequired, "A stream name must be supplied", http.StatusBadRequest)
			return
		}
		if !s.server.StreamExists(streamName) {
			s.server.CreateStream(streamName)
		}
		c := metrics2.GetCounter(clientConnectionsMetricName, map[string]string{"stream": streamName})
		c.Inc(1)
		s.server.ServeHTTP(w, r)
		c.Dec(1)
	}
}

// Send implements Server.
func (s *ServerImpl) Send(ctx context.Context, stream string, msg string) error {
	if msg == "" {
		return ErrOnlySendNoneEmptyMessages
	}

	s.sendCh <- Event{Stream: stream, Msg: msg}
	return nil
}

var _ Server = (*ServerImpl)(nil)
