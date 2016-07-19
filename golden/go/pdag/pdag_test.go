package pdag

import (
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
)

type extype struct {
	data  map[string]int
	mutex sync.Mutex
}

func TestSimpleTopology(t *testing.T) {
	rootFn := func(ctx interface{}) error {
		d := ctx.(*extype)
		d.data["val"] = 0
		return nil
	}

	sinkFn := func(ctx interface{}) error {
		d := ctx.(*extype)
		d.data["val2"] = d.data["val"] * 100
		return nil
	}

	// Create a two node topology with a source and a sink.
	root := NewNode(rootFn)
	root.Child(sinkFn)

	// Create a context and trigger in the root node.
	d := &extype{data: map[string]int{}}
	err := root.Trigger(d)

	assert.Nil(t, err)
	assert.Equal(t, 2, len(d.data))
	assert.Equal(t, d.data["val"], 0)
	assert.Equal(t, d.data["val2"], 0)
}

func TestGenericTopology(t *testing.T) {
	orderCh := make(chan string, 5)

	rootFn := func(ctx interface{}) error {
		d := ctx.(*extype)
		d.data["val"] = 0
		orderCh <- "a"
		return nil
	}

	aFn := incFn(1, orderCh, "b")
	bFn := incFn(10, orderCh, "c")
	cFn := incFn(100, orderCh, "d")

	sinkFn := func(ctx interface{}) error {
		d := ctx.(*extype)
		d.data["val2"] = d.data["val"] * 100
		orderCh <- "e"
		return nil
	}

	// Create a topology that fans out to aFn, bFn, cFn and
	// then collects the results in a sink function.
	root := NewNode(rootFn)
	NewNode(sinkFn,
		root.Child(aFn),
		root.Child(bFn),
		root.Child(cFn))

	// Create a context and trigger in the root node.
	d := &extype{data: map[string]int{}}
	start := time.Now()
	err := root.Trigger(d)
	delta := time.Now().Sub(start)

	assert.Nil(t, err)

	// Make sure the functions are called in parallel.
	assert.True(t, delta < (510*time.Millisecond))
	assert.Equal(t, len(d.data), 2)
	assert.Equal(t, len(orderCh), 5)
	assert.Equal(t, d.data["val"], 111)
	assert.Equal(t, d.data["val2"], 11100)

	// Make sure the functions are called in the right order.
	assert.Equal(t, <-orderCh, "a")
	parallel := []string{<-orderCh, <-orderCh, <-orderCh}
	sort.Strings(parallel)
	assert.Equal(t, []string{"b", "c", "d"}, parallel)
	assert.Equal(t, <-orderCh, "e")
}

func TestError(t *testing.T) {
	errFn := func(c interface{}) error {
		return fmt.Errorf("Not Implemented")
	}

	root := NewNode(NoOp)
	root.Child(NoOp).
		Child(NoOp).
		Child(errFn).
		Child(NoOp)

	err := root.Trigger(nil)
	assert.NotNil(t, err)
	assert.Equal(t, "Not Implemented", err.Error())
}

func incFn(increment int, ch chan<- string, chVal string) ProcessFn {
	return func(ctx interface{}) error {
		d := ctx.(*extype)
		d.mutex.Lock()
		d.data["val"] += increment
		d.mutex.Unlock()
		ch <- chVal
		time.Sleep(time.Millisecond * 500)
		return nil
	}
}
