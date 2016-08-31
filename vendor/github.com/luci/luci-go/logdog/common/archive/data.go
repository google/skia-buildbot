// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package archive

import (
	"io"

	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/common/renderer"
)

// channelRendererSource is a renderer.Source implementation that reads LogEntry
// records from a channel. It also 'sticks' on the first error that it
// encounters, so that all subsequent interface methods will permanently return
// the first encountered error.
//
// When the channel has closed, it will return io.EOF conformant to
// renderer.Source.
type channelRendererSource <-chan *logpb.LogEntry

func (cs channelRendererSource) NextLogEntry() (*logpb.LogEntry, error) {
	if le, ok := <-cs; ok {
		return le, nil
	}
	return nil, io.EOF
}

func archiveData(w io.Writer, dataC <-chan *logpb.LogEntry) error {
	entryC := make(chan *logpb.LogEntry)
	r := renderer.Renderer{
		Source:    channelRendererSource(entryC),
		Reproduce: true,
	}

	// Run our Renderer in a goroutine.
	rendererFinishedC := make(chan error)
	go func() {
		var err error
		defer func() {
			rendererFinishedC <- err
		}()

		// Render our stream.
		_, err = io.Copy(w, &r)

		// If our Renderer encounters an error, it will stop reading from entryC
		// prematurely. In order for the pipeline to complete, we need to manually
		// drain entryC here.
		for range entryC {
			// Discard.
		}
	}()

	// Consume records from dataC. If we hit an error condition, consume and
	// discard.
	for le := range dataC {
		entryC <- le
	}
	close(entryC)
	return <-rendererFinishedC
}
