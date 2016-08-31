// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package streamserver

import (
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"

	"github.com/luci/luci-go/common/iotools"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/runtime/paniccatcher"
	"github.com/luci/luci-go/logdog/client/butlerlib/streamproto"
	"golang.org/x/net/context"
)

// streamParams are parameters representing a negotiated stream ready to
// deliver.
type streamParams struct {
	// The stream's ReadCloser connection.
	rc io.ReadCloser
	// Negotiated stream properties.
	properties *streamproto.Properties
}

// listenerStreamServer is the class for Listener-based stream server
// implementations.
type listenerStreamServer struct {
	context.Context

	// gen is a generator function that is called to produce the stream server's
	// Listener. On success, it returns the instantiated Listener, which will be
	// closed when the stream server is closed.
	gen   func() (net.Listener, error)
	l     net.Listener
	laddr string

	streamParamsC   chan *streamParams
	closedC         chan struct{}
	acceptFinishedC chan struct{}

	// closed is an atomically-protected integer. If its value is 1, the stream
	// server has been closed.
	closed int32

	// discardC is a testing channel. If not nil, failed client connections will
	// be written here.
	discardC chan *streamClient
}

var _ StreamServer = (*listenerStreamServer)(nil)

func (s *listenerStreamServer) String() string {
	return fmt.Sprintf("%T(%s)", s, s.laddr)
}

func (s *listenerStreamServer) Listen() error {
	// Create a listener (OS-specific).
	var err error
	s.l, err = s.gen()
	if err != nil {
		return err
	}

	s.laddr = s.l.Addr().String()
	s.streamParamsC = make(chan *streamParams)
	s.closedC = make(chan struct{})
	s.acceptFinishedC = make(chan struct{})

	// Poll the Listener for new connections in a separate goroutine. This will
	// terminate when the server is Close()d.
	go s.serve()
	return nil
}

func (s *listenerStreamServer) Next() (io.ReadCloser, *streamproto.Properties) {
	if streamParams, ok := <-s.streamParamsC; ok {
		return streamParams.rc, streamParams.properties
	}
	return nil, nil
}

// Implements StreamServer.Close
func (s *listenerStreamServer) Close() {
	if s.l == nil {
		panic("server is not currently serving")
	}

	// Mark that we've been closed.
	atomic.StoreInt32(&s.closed, 1)

	// Close our Listener. This will cause our 'Accept' goroutine to terminate.
	s.l.Close()
	close(s.closedC)
	<-s.acceptFinishedC

	// Close our streamParamsC to signal that we're closed. Any blocking Next will
	// drain the channel, then return with nil.
	close(s.streamParamsC)
	s.l = nil
}

// Continuously pulls connections from the supplied Listener and returns them as
// connections to streamParamsC for consumption by Next().
func (s *listenerStreamServer) serve() {
	defer close(s.acceptFinishedC)

	nextID := 0
	clientWG := sync.WaitGroup{}
	for {
		log.Debugf(s, "Beginning Accept() loop cycle.")
		conn, err := s.l.Accept()
		if err != nil {
			if atomic.LoadInt32(&s.closed) != 0 {
				log.WithError(err).Debugf(s, "Error during Accept() encountered (closed).")
			} else {
				log.WithError(err).Errorf(s, "Error during Accept().")
			}
			break
		}

		// Spawn a goroutine to handle this connection. This goroutine will take
		// ownership of the connection, closing it as appropriate.
		client := &streamClient{
			closedC: s.closedC,
			id:      nextID,
			conn:    &iotools.DeadlineReader{conn, 0},
		}
		client.Context = log.SetFields(s, log.Fields{
			"id":     client.id,
			"local":  client.conn.LocalAddr(),
			"remote": client.conn.RemoteAddr(),
		})

		clientWG.Add(1)
		go func() {
			defer clientWG.Done()

			var params *streamParams
			paniccatcher.Do(func() {
				var err error
				params, err = client.handle()
				if err != nil {
					log.Fields{
						log.ErrorKey: err,
					}.Errorf(client, "Failed to negotitate stream client.")
					params = nil
				}
			}, func(p *paniccatcher.Panic) {
				log.Fields{
					"panicReason": p.Reason,
				}.Errorf(client, "Panic during client handshake:\n%s", p.Stack)
				params = nil
			})

			// Did the negotiation fail?
			if params != nil {
				// Punt the client to our Next function. If we close while waiting, close
				// the Client.
				select {
				case s.streamParamsC <- params:
					break

				case <-s.closedC:
					log.Warningf(client, "Closed with client in hand. Cleaning up client.")
					params.rc.Close()
					params = nil
				}
			}

			// (Testing) write failed connections to discardC if configured.
			if params == nil && s.discardC != nil {
				s.discardC <- client
			}
		}()
		nextID++
	}

	// Wait for client connections to finish.
	clientWG.Wait()
	log.Fields{
		"totalConnections": nextID,
	}.Infof(s, "Exiting serve loop.")
}

// streamClient manages a single client connection.
type streamClient struct {
	context.Context

	closedC chan struct{} // Signal channel to indicate that the server has closed.
	id      int           // Client ID, used for debugging correlation.
	conn    net.Conn      // The underlying client connection.

	// decoupleMu is used to ensure that decoupleConn is called at most one time.
	decoupleMu sync.Mutex
}

// handle initializes the supplied connection. On success, it will prepare
// the connection as a Butler stream and pass its parameters through the
// supplied channel.
//
// On failure, the connection will be closed.
//
// This method returns the negotiated stream parameters, or nil if the
// negotitation failed and the client was closed.
func (c *streamClient) handle() (*streamParams, error) {
	log.Infof(c, "Received new connection.")

	// Close the connection as a failsafe. If we have already decoupled it, this
	// will end up being a no-op.
	defer c.closeConn()

	// Perform our handshake. We pass the connection explicitly into this method
	// because it can get decoupled during operation.
	p, err := handshake(c, c.conn)
	if err != nil {
		return nil, err
	}

	// Successful handshake. Forward our properties.
	return &streamParams{c.decoupleConn(), p}, nil
}

// handshake handles the handshaking and registration of a single connection. If
// the connection successfully handshakes, it will be registered as a stream and
// supplied to the local streamParamsC; otherwise, it will be closed.
//
// The client connection opens with a handshake protocol. Once complete, the
// connection itself becomes the stream.
func handshake(ctx context.Context, conn net.Conn) (*streamproto.Properties, error) {
	log.Infof(ctx, "Beginning handshake.")
	hs := handshakeProtocol{}
	return hs.Handshake(ctx, conn)
}

// Closes the underlying connection.
func (c *streamClient) closeConn() {
	conn := c.decoupleConn()
	if conn != nil {
		if err := conn.Close(); err != nil {
			log.Fields{
				log.ErrorKey: err,
			}.Warningf(c, "Error on connection close.")
		}
	}
}

// Decouples the active connection, returning it and setting the connection to
// nil.
func (c *streamClient) decoupleConn() (conn net.Conn) {
	c.decoupleMu.Lock()
	defer c.decoupleMu.Unlock()

	conn, c.conn = c.conn, nil
	return
}
