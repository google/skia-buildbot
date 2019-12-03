package pdag

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
)

// The simulated duration of each function in ms.
const fnDuration = 500 * time.Millisecond

type extype struct {
	data  map[string]int
	mutex sync.Mutex
}

func TestSimpleTopology(t *testing.T) {
	unittest.SmallTest(t)
	rootFn := func(_ context.Context, state interface{}) error {
		d := state.(*extype)
		d.data["val"] = 0
		return nil
	}

	sinkFn := func(_ context.Context, state interface{}) error {
		d := state.(*extype)
		d.data["val2"] = d.data["val"] * 100
		return nil
	}

	// Create a two node topology with a source and a sink.
	root := NewNodeWithParents(rootFn)
	root.Child(sinkFn)

	// Create a context and trigger in the root node.
	d := &extype{data: map[string]int{}}
	err := root.Trigger(context.Background(), d)

	require.Nil(t, err)
	require.Equal(t, 2, len(d.data))
	require.Equal(t, d.data["val"], 0)
	require.Equal(t, d.data["val2"], 0)
}

func TestGenericTopology(t *testing.T) {
	unittest.SmallTest(t)
	orderCh := make(chan string, 5)

	rootFn := func(_ context.Context, state interface{}) error {
		d := state.(*extype)
		d.data["val"] = 0
		orderCh <- "a"
		return nil
	}

	bFn := incFn(1, orderCh, "b")
	cFn := incFn(10, orderCh, "c")
	dFn := incFn(100, orderCh, "d")

	sinkFn := func(_ context.Context, state interface{}) error {
		d := state.(*extype)
		d.data["val2"] = d.data["val"] * 100
		orderCh <- "e"
		return nil
	}

	// Create a topology that fans out to aFn, bFn, cFn and
	// then collects the results in a sink function.
	root := NewNodeWithParents(rootFn)
	NewNodeWithParents(sinkFn,
		root.Child(bFn),
		root.Child(cFn),
		root.Child(dFn))

	// Create a context and trigger in the root node.
	d := &extype{data: map[string]int{}}
	start := time.Now()
	err := root.Trigger(context.Background(), d)
	delta := time.Since(start)

	require.Nil(t, err)

	// Make sure the functions are roughly called in parallel.
	require.True(t, delta < (2*fnDuration))
	require.Equal(t, len(d.data), 2)
	require.Equal(t, len(orderCh), 5)
	require.Equal(t, d.data["val"], 111)
	require.Equal(t, d.data["val2"], 11100)

	// Make sure the functions are called in the right order.
	require.Equal(t, <-orderCh, "a")
	parallel := []string{<-orderCh, <-orderCh, <-orderCh}
	sort.Strings(parallel)
	require.Equal(t, []string{"b", "c", "d"}, parallel)
	require.Equal(t, <-orderCh, "e")
}

func TestError(t *testing.T) {
	unittest.SmallTest(t)
	errFn := func(_ context.Context, _ interface{}) error {
		return fmt.Errorf("Not Implemented")
	}

	root := NewNodeWithParents(NoOp)
	root.Child(NoOp).
		Child(NoOp).
		Child(errFn).
		Child(NoOp)

	err := root.Trigger(context.Background(), nil)
	require.Error(t, err)
	require.Equal(t, "Not Implemented", err.Error())
}

func TestCancelledContext(t *testing.T) {
	unittest.SmallTest(t)
	errFn := func(_ context.Context, _ interface{}) error {
		assert.Fail(t, "should not be called")
		return nil
	}

	root := NewNodeWithParents(NoOp)
	root.Child(NoOp).
		Child(NoOp).
		Child(errFn).
		Child(NoOp)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := root.Trigger(ctx, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "canceled")
}

func TestComplexCallOrder(t *testing.T) {
	unittest.SmallTest(t)
	aFn := orderFn("a")
	bFn := orderFn("b")
	cFn := orderFn("c")
	dFn := orderFn("d")
	eFn := orderFn("e")
	fFn := orderFn("f")
	gFn := orderFn("g")

	a := NewNodeWithParents(aFn).setName("a")
	b := a.Child(bFn).setName("b")
	c := a.Child(cFn).setName("c")
	b.Child(dFn).setName("d")
	e := NewNodeWithParents(eFn, b, c).setName("e")
	NewNodeWithParents(fFn, b, e).setName("f")
	e.Child(gFn).setName("g")

	// Create a context and trigger in the root node.
	data := make(chan string, 100)
	a.verbose = true
	require.NoError(t, a.Trigger(context.Background(), data))
	close(data)
	o := ""
	for c := range data {
		o += c
	}
	bPos := strings.Index(o, "b")
	dPos := strings.Index(o, "d")
	require.True(t, (bPos >= 0) && (dPos > bPos))

	// make sure d is called after b
	results := map[string]bool{
		"abcefg": true,
		"abcegf": true,
		"acbefg": true,
		"acbegf": true,
	}
	o = o[0:dPos] + o[dPos+1:]
	require.True(t, results[o])

	// Enumerate the possible outcome and count how often each occurs.
	posOutcome := []string{"bdegf", "bdefg", "bedgf", "bedfg", "befdg", "begdf", "begfd", "befgd"}
	expSet := util.NewStringSet(posOutcome)
	require.Equal(t, len(posOutcome), len(expSet))

	// Make a call an node in the DAG and make the call order works.
	data = make(chan string, 100)
	b.verbose = true
	require.NoError(t, b.Trigger(context.Background(), data))
	close(data)
	o = ""
	for c := range data {
		o += c
	}

	require.True(t, expSet[o], "Instead got: "+o)
}

func orderFn(msg string) ProcessFn {
	return func(_ context.Context, state interface{}) error {
		state.(chan string) <- msg
		return nil
	}
}

func incFn(increment int, ch chan<- string, chVal string) ProcessFn {
	return func(_ context.Context, state interface{}) error {
		d := state.(*extype)
		d.mutex.Lock()
		d.data["val"] += increment
		d.mutex.Unlock()
		ch <- chVal
		time.Sleep(fnDuration)
		return nil
	}
}
