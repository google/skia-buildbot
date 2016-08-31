// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package streamclient

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/luci/luci-go/logdog/client/butlerlib/streamproto"
)

// Client is a client to a LogDog Butler StreamServer. A Client will connect
// to a StreamServer, negotiate a stream configuration, and return an active
// stream object that can be written to.
type Client interface {
	// NewStream creates a new stream with the supplied stream properties.
	NewStream(f streamproto.Flags) (Stream, error)
}

// streamFactory is a factory method to generate an io.WriteCloser stream for
// the current clientImpl.
type streamFactory func() (io.WriteCloser, error)

// clientImpl is an implementation of the Client interface using a net.Conn
// factory to generate an individual stream.
type clientImpl struct {
	// network is the connection path to the stream server.
	factory streamFactory
}

// New instantiates a new Client instance. This type of instance will be parsed
// from the supplied path string, which takes the form:
//   <protocol>:<protocol-specific-spec>
//
// Supported protocols and their respective specs are:
//   - unix:/path/to/socket describes a stream server listening on UNIX domain
//     socket at "/path/to/socket".
//
// Windows-only:
//   - net.pipe:name describes a stream server listening on Windows named pipe
//     "\\.\pipe\name".
func New(path string) (Client, error) {
	return DefaultRegistry.NewClient(path)
}

func (c *clientImpl) NewStream(f streamproto.Flags) (Stream, error) {
	p := f.Properties()
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("streamclient: invalid stream properties: %s", err)
	}

	client, err := c.factory()
	if err != nil {
		return nil, err
	}
	ownsClient := true
	defer func() {
		// If we haven't written out the connection, close.
		if ownsClient {
			client.Close()
		}
	}()

	data, err := json.Marshal(f)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal properties JSON: %s", err)
	}

	// Perform the handshake: magic + size(data) + data.
	s := &streamImpl{
		Properties:  p,
		WriteCloser: client,
	}
	if _, err := s.writeRaw(streamproto.ProtocolFrameHeaderMagic); err != nil {
		return nil, fmt.Errorf("failed to write magic number: %s", err)
	}
	if err := s.writeRecord(data); err != nil {
		return nil, fmt.Errorf("failed to write properties: %s", err)
	}

	ownsClient = false // Passing ownership to caller.
	return s, nil
}
