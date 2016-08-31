// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package butler

import (
	"io"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/iotools"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/logdog/client/butler/bundler"
	"golang.org/x/net/context"
)

type stream struct {
	context.Context

	r  io.Reader
	c  io.Closer
	bs bundler.Stream
}

func (s *stream) readChunk() bool {
	d := s.bs.LeaseData()
	defer func() {
		if d != nil {
			d.Release()
		}
	}()

	amount, err := s.r.Read(d.Bytes())
	if amount > 0 {
		d.Bind(amount, clock.Now(s))

		// Add the data to our bundler endpoint. This may block waiting for the
		// bundler to consume data.
		log.Fields{
			"numBytes": amount,
		}.Debugf(s, "Read byte(s).")
		if err := s.bs.Append(d); err != nil {
			log.Fields{
				log.ErrorKey: err,
			}.Errorf(s, "Failed to Append to stream.")
			return false
		}
		d = nil
	}

	switch err {
	case iotools.ErrTimeout:
		log.Debugf(s, "Encountered 'Read()' timeout; re-reading.")
		fallthrough
	case nil:
		break

	case io.EOF:
		log.Fields{
			log.ErrorKey: err,
		}.Infof(s, "Stream encountered EOF.")
		return false

	default:
		log.Fields{
			log.ErrorKey: err,
		}.Errorf(s, "Stream encountered error during Read.")
		return false
	}

	return true
}

func (s *stream) closeStream() {
	if err := s.c.Close(); err != nil {
		log.Fields{
			log.ErrorKey: err,
		}.Warningf(s, "Error closing stream.")
	}
	s.bs.Close()
}
