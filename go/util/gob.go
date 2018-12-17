package util

import (
	"bytes"
	"encoding/gob"
	"sync"
)

const kNumDecoderGoroutines = 10

// GobEncoder encodes structs into bytes via GOB encoding. Not safe for
// concurrent use.
//
// Here's a template for writing a type-specific encoder:
//
// // FooEncoder encodes Foos into bytes via GOB encoding. Not safe for
// // concurrent use.
// type FooEncoder {
// 	util.GobEncoder
// }
//
// // Next returns one of the Foox provided to Process (in arbitrary order) and
// // its serialized bytes. If any items remain, returns the item, the
// // serialized bytes, nil. If all items have been returned, returns nil, nil,
// // nil. If an error is encountered, returns nil, nil, error.
// func (e *FooEncoder) Next() (*Foo, []byte, error) {
//	item, serialized, err := e.GobEncoder.Next()
//	if err != nil {
//		return nil, nil, err
//	} else if item == nil {
//		return nil, nil, nil
//	}
//	return item.(*Foo), serialized, nil
// }
type GobEncoder struct {
	err    error
	items  []interface{}
	result [][]byte
}

// Process encodes the item into a byte slice that will be returned from
// Next() (in arbitrary order). Returns false if Next is certain to return an
// error. Caller must ensure item does not change until after the first call to
// Next(). May not be called after calling Next().
func (e *GobEncoder) Process(item interface{}) bool {
	if e.err != nil {
		return false
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(item); err != nil {
		e.err = err
		e.items = nil
		e.result = nil
		return false
	}
	e.items = append(e.items, item)
	e.result = append(e.result, buf.Bytes())
	return true
}

// Next returns one of the items provided to Process (in arbitrary order) and
// its serialized bytes. If any items remain, returns the item, the serialized
// bytes, nil. If all items have been returned, returns nil, nil, nil. If an
// error is encountered, returns nil, nil, error.
func (e *GobEncoder) Next() (interface{}, []byte, error) {
	if e.err != nil {
		return nil, nil, e.err
	}
	if len(e.items) == 0 {
		return nil, nil, nil
	}
	c := e.items[0]
	e.items = e.items[1:]
	serialized := e.result[0]
	e.result = e.result[1:]
	return c, serialized, nil
}

// GobDecoder decodes bytes into structs via GOB decoding. Not safe for
// concurrent use.
//
// Here's a template for writing a type-specific decoder:
//
// FooDecoder decodes bytes into Foos via GOB decoding. Not safe for
// concurrent use.
// type FooDecoder struct {
//	*util.GobDecoder
// }
//
// // NewFooDecoder returns a FooDecoder instance.
// func NewFooDecoder() *FooDecoder {
//	return &FooDecoder{
//		GobDecoder: util.NewGobDecoder(func() interface{} {
//			return &Foo{}
//		}, func(ch <-chan interface{}) interface{} {
//			items := []*Foo{}
//			for item := range ch {
//				items = append(items, item.(*Foo))
//			}
//			return items
//		}),
//	}
// }
//
// // Result returns all decoded Foos provided to Process (in arbitrary order), or
// // any error encountered.
// func (d *FooDecoder) Result() ([]*Foo, error) {
//	res, err := d.GobDecoder.Result()
//	if err != nil {
//		return nil, err
//	}
//	return res.([]*Foo), nil
// }
type GobDecoder struct {
	// input contains the incoming byte slices. Process() sends on this
	// channel, decode() receives from it, and Result() closes it.
	input chan []byte
	// output contains decoded items. decode() sends on this channel,
	// collect() receives from it, and run() closes it when all decode()
	// goroutines have finished.
	output chan interface{}
	// result contains the return value of Result(). collect() sends a single
	// value on this channel and closes it. Result() receives from it.
	result chan interface{}
	// errors contains the first error from any goroutine. It's a channel in
	// case multiple goroutines experience an error at the same time.
	errors chan error

	newItem     func() interface{}
	collectImpl func(<-chan interface{}) interface{}
}

// NewGobDecoder returns a GobDecoder instance. The first argument is a
// goroutine-safe function which returns a zero-valued instance of the type
// being decoded, eg.
//
// func() interface{} {
//	return &MyType{}
// }
//
// The second argument is a function which collects decoded instances of that
// type from a channel and returns a slice, eg.
//
// func(ch <-chan interface{}) interface{} {
//	items := []*MyType{}
//	for item := range ch {
//		items = append(items, item.(*MyType))
//	}
//	return items
// }
func NewGobDecoder(newItem func() interface{}, collect func(<-chan interface{}) interface{}) *GobDecoder {
	d := &GobDecoder{
		input:       make(chan []byte, kNumDecoderGoroutines*2),
		output:      make(chan interface{}, kNumDecoderGoroutines),
		result:      make(chan interface{}, 1),
		errors:      make(chan error, kNumDecoderGoroutines),
		newItem:     newItem,
		collectImpl: collect,
	}
	go d.run()
	go d.collect()
	return d
}

// run starts the decode goroutines and closes d.output when they finish.
func (d *GobDecoder) run() {
	// Start decoders.
	wg := sync.WaitGroup{}
	for i := 0; i < kNumDecoderGoroutines; i++ {
		wg.Add(1)
		go d.decode(&wg)
	}
	// Wait for decoders to exit.
	wg.Wait()
	// Drain d.input in the case that errors were encountered, to avoid deadlock.
	for range d.input {
	}
	close(d.output)
}

// decode receives from d.input and sends to d.output until d.input is closed or
// d.errors is non-empty. Decrements wg when done.
func (d *GobDecoder) decode(wg *sync.WaitGroup) {
	for b := range d.input {
		item := d.newItem()
		if err := gob.NewDecoder(bytes.NewReader(b)).Decode(item); err != nil {
			d.errors <- err
			break
		}
		d.output <- item
		if len(d.errors) > 0 {
			break
		}
	}
	wg.Done()
}

// collect receives from d.output until it is closed, then sends on d.result.
func (d *GobDecoder) collect() {
	d.result <- d.collectImpl(d.output)
	close(d.result)
}

// Process decodes the byte slice and includes it in Result() (in arbitrary
// order). Returns false if Result is certain to return an error. Caller must
// ensure b does not change until after Result() returns.
func (d *GobDecoder) Process(b []byte) bool {
	d.input <- b
	return len(d.errors) == 0
}

// Result returns all decoded items provided to Process (in arbitrary order), or
// any error encountered.
func (d *GobDecoder) Result() (interface{}, error) {
	close(d.input)
	select {
	case err := <-d.errors:
		return nil, err
	case result := <-d.result:
		return result, nil
	}
}
