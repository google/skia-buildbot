// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package swarming

import (
	"bytes"
	"fmt"

	"github.com/golang/protobuf/proto"
	miloProto "github.com/luci/luci-go/common/proto/milo"
	"github.com/luci/luci-go/logdog/client/butlerlib/streamclient"
	"github.com/luci/luci-go/logdog/client/butlerlib/streamproto"
	"github.com/luci/luci-go/milo/appengine/logdog"
)

// In-memory datastructure to hold a fake butler client.
type memoryStream struct {
	*streamproto.Properties

	closed     bool
	buf        bytes.Buffer
	isDatagram bool
}

func (s *memoryStream) ToLogDogStream() (*logdog.Stream, error) {
	result := &logdog.Stream{
		Closed:     s.closed,
		IsDatagram: s.isDatagram,
		Path:       s.Name,
		Prefix:     s.Prefix,
	}

	if s.isDatagram {
		result.Data = &miloProto.Step{}
		// Assume this is a miloProto.Step.
		if err := proto.Unmarshal(s.buf.Bytes(), result.Data); err != nil {
			return nil, err
		}
	} else {
		result.Text = s.buf.String()
	}

	return result, nil
}

func (s *memoryStream) Write(b []byte) (int, error) {
	return s.buf.Write(b)
}

func (s *memoryStream) Close() error {
	s.closed = true
	return nil
}

func (s *memoryStream) WriteDatagram(b []byte) error {
	s.isDatagram = true

	s.buf.Reset()
	_, err := s.buf.Write(b)
	return err
}

type memoryClient struct {
	stream map[string]*memoryStream
}

func (c *memoryClient) NewStream(f streamproto.Flags) (streamclient.Stream, error) {
	props := f.Properties()
	if _, ok := c.stream[props.Name]; ok {
		return nil, fmt.Errorf("duplicate stream, %q", props.Name)
	}
	s := memoryStream{
		Properties: props,
	}
	if c.stream == nil {
		c.stream = map[string]*memoryStream{}
	}
	c.stream[s.Name] = &s
	return &s, nil
}

func (c *memoryClient) addLogDogTextStream(s *logdog.Streams, ls *miloProto.LogdogStream) error {
	var keys []string
	for k := range c.stream {
		keys = append(keys, k)
	}
	ms, ok := c.stream[ls.Name]
	if !ok {
		return fmt.Errorf("Could not find text stream %q\n%s", ls.Name, keys)
	}
	lds, err := ms.ToLogDogStream()
	if err != nil {
		return fmt.Errorf("Could not convert text stream %s\n%s\n%s", ls.Name, err, keys)
	}
	if lds.IsDatagram {
		return fmt.Errorf("Expected %s to be a text stream, got a data stream", ls.Name)
	}
	s.Streams[ls.Name] = lds
	return nil
}

// addToStreams adds the set of stream with a given base path to the logdog
// stream map.  A base path is assumed to have a stream named "annotations".
func (c *memoryClient) addToStreams(s *logdog.Streams, anno *miloProto.Step) error {
	if lds := anno.StdoutStream; lds != nil {
		if err := c.addLogDogTextStream(s, lds); err != nil {
			return fmt.Errorf(
				"Encountered error while processing step streams for STDOUT: %s", err)
		}
	}
	if lds := anno.StderrStream; lds != nil {
		if err := c.addLogDogTextStream(s, lds); err != nil {
			return fmt.Errorf(
				"Encountered error while processing step streams for STDERR: %s", err)
		}
	}

	// Look for substream.
	for _, link := range anno.GetOtherLinks() {
		lds := link.GetLogdogStream()
		// Not a logdog stream.
		if lds == nil {
			continue
		}

		if err := c.addLogDogTextStream(s, lds); err != nil {
			return fmt.Errorf(
				"Encountered error while processing step streams for '%s'\n%s", lds.Name, err)
		}
	}

	// Now do substeps.
	for _, subStepEntry := range anno.Substep {
		substep := subStepEntry.GetStep()
		if substep == nil {
			continue
		}

		if err := c.addToStreams(s, substep); err != nil {
			return err
		}
	}

	return nil
}

func (c *memoryClient) ToLogDogStreams() (*logdog.Streams, error) {
	result := &logdog.Streams{}
	result.Streams = map[string]*logdog.Stream{}

	// Register annotation stream.
	const annotationStreamName = "annotations"
	ms, ok := c.stream[annotationStreamName]
	if !ok {
		return nil, fmt.Errorf("Could not find stream %s", annotationStreamName)
	}
	ls, err := ms.ToLogDogStream()
	if err != nil {
		return nil, fmt.Errorf("Could not unmarshal stream %s\n%s", annotationStreamName, err)
	}
	if !ls.IsDatagram {
		return nil, fmt.Errorf(
			"Annotation stream %s is not a datagram\nText: %s",
			annotationStreamName, ls.Text)
	}
	result.Streams[annotationStreamName] = ls

	// Register any referenced LogDog streams.
	if err := c.addToStreams(result, ls.Data); err != nil {
		return nil, err
	}
	result.MainStream = ls

	if len(c.stream) != len(result.Streams) {
		var mk, lk []string
		for k := range c.stream {
			mk = append(mk, k)
		}
		for k := range result.Streams {
			lk = append(lk, k)
		}
		return nil, fmt.Errorf(
			"Number of streams do not match %d vs %d\nMemory:%s\nResult:%s",
			len(c.stream), len(result.Streams), mk, lk)
	}

	return result, nil
}
