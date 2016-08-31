// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package file

import (
	"os"
	"sort"
	"sync"

	"github.com/golang/protobuf/proto"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/client/butler/output"
	"github.com/luci/luci-go/logdog/common/types"
	"golang.org/x/net/context"
)

// Options is the set of configuration options for the Output.
type Options struct {
	// Path is the output file path.
	Path string
	// Track, if true, causes log entry output to be tracked.
	Track bool
}

// New creates a new file Output from the specified Options.
func (opt Options) New(c context.Context) output.Output {
	o := fileOutput{
		Context: c,
		Options: &opt,
		streams: map[types.StreamPath]*stream{},
	}
	if opt.Track {
		o.et = &output.EntryTracker{}
	}
	return &o
}

// fileOutput is an output.Output implementation that writes log stream data to
// on-disk files.
type fileOutput struct {
	// Context is the context to use for logging. If nil, no logging will be
	// performed.
	context.Context
	// Options are the configuration options.
	*Options
	// Mutex protects all other members.
	sync.Mutex

	// streams is a map of stream name to stream handler.
	streams map[types.StreamPath]*stream
	// stats is the streaming stats for this instance.
	stats output.StatsBase
	// et is the singleton EntryTracker.
	et *output.EntryTracker
}

func (o *fileOutput) SendBundle(b *logpb.ButlerLogBundle) error {
	o.Lock()
	defer o.Unlock()

	for _, be := range b.GetEntries() {
		desc := be.GetDesc()
		if desc == nil {
			continue
		}
		path := desc.Path()

		s, ok := o.streams[path]
		if !ok {
			s = newStream(desc)
			o.streams[path] = s
		}

		s.ingestBundleEntry(be)
	}

	if o.et != nil {
		o.et.Track(b)
	}
	return nil
}

func (o *fileOutput) MaxSize() int {
	return 1024 * 1024 * 1024
}

func (o *fileOutput) Stats() output.Stats {
	o.Lock()
	defer o.Unlock()

	out := o.stats
	for _, st := range o.streams {
		out.Merge(&st.stats)
	}
	return &out
}

func (o *fileOutput) Record() *output.EntryRecord {
	o.Lock()
	defer o.Unlock()

	if o.et == nil {
		return nil
	}
	return o.et.Record()
}

func (o *fileOutput) Close() {
	o.Lock()
	defer o.Unlock()

	if o.streams == nil {
		panic("already closed")
	}

	b := o.getBundleLocked()
	o.stats.F.SentBytes += int64(proto.Size(b))
	o.stats.F.SentMessages++

	err := func() error {
		fd, err := os.Create(o.Path)
		if err != nil {
			return err
		}
		defer fd.Close()

		var tm proto.TextMarshaler
		if err := tm.Marshal(fd, b); err != nil {
			return err
		}
		return nil
	}()
	if err != nil {
		log.Fields{
			log.ErrorKey: err,
			"path":       o.Path,
		}.Errorf(o, "Failed to write file output.")
		o.stats.F.Errors++
	}
}

func (o *fileOutput) getBundleLocked() *logpb.ButlerLogBundle {
	b := logpb.ButlerLogBundle{
		Entries: make([]*logpb.ButlerLogBundle_Entry, len(o.streams)),
	}

	streamNames := make([]string, 0, len(o.streams))
	for name := range o.streams {
		streamNames = append(streamNames, string(name))
	}
	sort.Strings(streamNames)

	for i, name := range streamNames {
		b.Entries[i] = o.streams[types.StreamPath(name)].getBundleEntry()
	}
	o.streams = nil
	return &b
}
