// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bundler

import (
	"sync"
	"time"

	"github.com/luci/luci-go/common/data/chunkstream"
)

type dataPoolRegistry struct {
	sync.Mutex

	// pools is a pool of Data instances. It is keyed on buffer size.
	pools map[int]*dataPool
}

// globaldataPoolRegistry is the default data pool to use by the stream
// package.
var globalDataPoolRegistry = dataPoolRegistry{}

func (r *dataPoolRegistry) getPool(s int) *dataPool {
	r.Lock()
	defer r.Unlock()

	if r.pools == nil {
		r.pools = map[int]*dataPool{}
	}

	pool := r.pools[s]
	if pool == nil {
		pool = newPool(s)
		r.pools[s] = pool
	}
	return pool
}

type dataPool struct {
	sync.Pool
	size int
}

func newPool(size int) *dataPool {
	p := dataPool{
		size: size,
	}
	p.New = p.newData
	return &p
}

func (p *dataPool) newData() interface{} {
	return &streamData{
		bufferBase:  make([]byte, p.size),
		releaseFunc: p.release,
	}
}

func (p *dataPool) getData() Data {
	d := p.Get().(*streamData)
	d.reset()
	return d
}

func (p *dataPool) release(d *streamData) {
	p.Put(d)
}

// Data is a reusable data buffer that is used by Stream instances to ingest
// data.
//
// Data is initially an empty buffer. Once data is loaded into it, the buffer is
// resized to the bound data and a timestamp is attached via Bind.
type Data interface {
	chunkstream.Chunk

	// Bind resizes the Chunk buffer and records a timestamp to associate with the
	// data chunk.
	Bind(int, time.Time) Data

	// Timestamp returns the bound timestamp. This will be zero if no timestamp
	// has been bound.
	Timestamp() time.Time
}

// streamData is an implementation of the Chunk interface for Bundler chunks.
//
// It includes the ability to bind to a size/timestamp.
type streamData struct {
	bufferBase []byte
	buffer     []byte
	ts         time.Time

	releaseFunc func(*streamData)
}

var _ Data = (*streamData)(nil)

func (d *streamData) reset() {
	d.buffer = d.bufferBase
}

func (d *streamData) Bytes() []byte {
	return d.buffer
}

func (d *streamData) Len() int {
	return len(d.buffer)
}

func (d *streamData) Bind(amount int, ts time.Time) Data {
	d.buffer = d.buffer[:amount]
	d.ts = ts
	return d
}

func (d *streamData) Timestamp() time.Time {
	return d.ts
}

func (d *streamData) Release() {
	if d.releaseFunc != nil {
		d.releaseFunc(d)
	}
}
