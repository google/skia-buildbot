// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/common/proto/milo"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/client/butlerlib/streamclient"
	"github.com/luci/luci-go/logdog/client/butlerlib/streamproto"
)

// filesystemClient is a streamproto.Client implementation that writes generated
// streams to files in a directory.
type filesystemClient struct {
	dir string

	fnIdxMu sync.Mutex
	fnIdx   map[string]int
}

func newFilesystemClient(dir string) (streamclient.Client, error) {
	// Create the directory if it doesn't exist.
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	return &filesystemClient{
		dir:   dir,
		fnIdx: map[string]int{},
	}, nil
}

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsNumber(r)) {
			return r
		}
		return '_'
	}, s)
}

func (c *filesystemClient) nextFilenameIndex(base string) int {
	c.fnIdxMu.Lock()
	defer c.fnIdxMu.Unlock()

	idx := c.fnIdx[base]
	c.fnIdx[base] = idx + 1
	return idx
}

func (c *filesystemClient) getFilename(base, ext string) string {
	idx := c.nextFilenameIndex(base)

	path := ""
	if idx == 0 {
		path = fmt.Sprintf("%s.%s", base, ext)
	} else {
		path = fmt.Sprintf("%s.%d.%s", base, idx, ext)
	}
	return path
}

func (c *filesystemClient) NewStream(f streamproto.Flags) (streamclient.Stream, error) {
	s := filesystemClientStream{
		filesystemClient: c,
		baseName:         sanitize(string(f.Name)),
		contentType:      f.ContentType,
		streamType:       logpb.StreamType(f.Type),
	}

	// Open our output file for writing.
	return &s, nil
}

type filesystemClientStream struct {
	*filesystemClient

	baseName    string
	contentType string
	streamType  logpb.StreamType

	writer io.WriteCloser
	dgIdx  int
}

func (s *filesystemClientStream) create(filename string) (io.WriteCloser, error) {
	path := filepath.Join(s.dir, filename)
	fd, err := os.OpenFile(path, (os.O_WRONLY | os.O_CREATE | os.O_TRUNC), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create file [%s]: %v", path, err)
	}
	return fd, nil
}

func (s *filesystemClientStream) Write(d []byte) (int, error) {
	if s.writer == nil {
		filename := ""
		switch s.streamType {
		case logpb.StreamType_TEXT:
			filename = s.getFilename(s.baseName, "txt")

		default:
			filename = s.getFilename(s.baseName, "bin")
		}

		w, err := s.create(filename)
		if err != nil {
			return 0, err
		}
		s.writer = w
	}

	return s.writer.Write(d)
}

func (s *filesystemClientStream) WriteDatagram(dg []byte) error {
	index := s.dgIdx
	s.dgIdx++

	if s.contentType == string(milo.ContentTypeAnnotations) {
		// If we successfully dump as Milo proto, yay.
		if err := s.dumpMiloProto(dg, index); err == nil {
			return nil
		}
	}

	// Dump as binary.
	w, err := s.create(s.getFilename(fmt.Sprintf("%s.%d", s.baseName, index), "bin"))
	if err != nil {
		return err
	}
	defer w.Close()

	_, err = w.Write(dg)
	return err
}

func (s *filesystemClientStream) dumpMiloProto(dg []byte, index int) error {
	// Dump this as a Milo annotation.
	ms := milo.Step{}
	if err := proto.Unmarshal(dg, &ms); err != nil {
		return err
	}
	// Successfully projected into milo.Step, dump as text!
	w, err := s.create(s.getFilename(fmt.Sprintf("%s.%d.annotations", s.baseName, index), "txt"))
	if err != nil {
		return err
	}
	defer w.Close()

	if err := proto.MarshalText(w, &ms); err != nil {
		return fmt.Errorf("failed to marshal Milo proto: %v", err)
	}
	return nil
}

func (s *filesystemClientStream) Close() error {
	if w := s.writer; w != nil {
		s.writer = nil
		return w.Close()
	}
	return nil
}
