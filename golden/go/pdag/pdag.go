// Package pdag allows to define a DAG of processing fuctions.
//
// The processing can be triggered at any node of the graph
// and will follow the directed edges of the graph. Before
// a function in a node is executed all its parents
// have to finish execution. Thus the graph defines the order
// in which the functions are executed and which functions are
// executed serially or in parallel.
//
// A single shared object (of type interface{}) is passed
// to all functions. It's the responsibility of the functions
// to coordinate synchronized access to the object.
//
// If an error occurs in any functions all processing seizes
// and the error is returned by the Trigger function.
//
// For example
//
//     root := NewNodeWithParents(a)
//     NewNodeWithParents(d, root.Child(b), root.Child(c))
//
//     state := map[string]string{}
//     root.Trigger(data)
//
// defines a diamond-shaped DAG. Execution starts at the root
// and after function 'a', functions 'b' and 'c' will be executed
// in parallel. Once both have completed, function 'd' will be
// called. All will be passed the value of 'state'.
//
// An instance of processing DAG is thread-safe. Data consistency has
// to be ensured in the shared state.
package pdag

import (
	"sync"

	"github.com/google/uuid"
	"go.skia.org/infra/go/sklog"
)

// Type of the processing functio nin each node.
type ProcessFn func(interface{}) error

// Node of the Dag.
type Node struct {
	id       string
	name     string
	children map[string]*Node
	procFn   ProcessFn
	mutex    sync.Mutex
	inputMap map[string]int
	inputCh  chan *call
	verbose  bool // debuging only
}

// Processing function that does nothing. Mostly used for testing.
func NoOp(ctx interface{}) error {
	return nil
}

// NewNodeWithParents creates a new Node in the processing DAG. It takes the function
// to be executed in this node and an optional list of parent nodes.
func NewNodeWithParents(fn ProcessFn, parents ...*Node) *Node {
	// Create a new node with a unique id.
	id := uuid.New()
	node := &Node{
		id:       id.String(),
		name:     id.String(),
		children: map[string]*Node{},
		procFn:   fn,
		inputCh:  make(chan *call),
		inputMap: map[string]int{},
	}

	// Link the children and parents.
	for _, parent := range parents {
		parent.children[node.id] = node
	}

	// Start the receiver for this node.
	go node.receiver()
	return node
}

// Child is a shorthand function that creates a child
// node of an existing node.
func (n *Node) Child(fn ProcessFn) *Node {
	return NewNodeWithParents(fn, n)
}

// Trigger starts execution at the current node and
// executes all functions that are descendents of this node.
// It blocks until all nodes have been executed. If any
// of the functions returns an error, execution ceases
// and the error is returned.
// Note: Trigger can be called on any node in the graph
// and will only call the decendents of that node.
func (n *Node) Trigger(state interface{}) error {
	// Create a call message.
	msg := call{
		id:    uuid.New().String(),
		state: state,
		errCh: make(chan error, 1),
	}

	// Mark all nodes with the number of inputs they should expect.
	nodesCalled := n.addInput(msg.id)
	msg.wg.Add(nodesCalled)

	if n.verbose {
		n.dump(msg.id, "")
		sklog.Infof("Number of nodes to call: %d\n", nodesCalled)
	}

	// Trigger the execution and wait for all nodes to be visited.
	n.inputCh <- &msg
	msg.wg.Wait()

	if msg.hasErr() {
		return <-msg.errCh
	}

	return nil
}

// setName assigns a name to the Node. It's purely used
// for debugging purposes. Iternally a unique id is used.
// it returns Node so it can easily be chained.
func (n *Node) setName(name string) *Node {
	n.name = name
	return n
}

// dump outputs the input connections of this node and its
// decendents. Only useful for debugging.
func (n *Node) dump(msgID, indent string) {
	sklog.Infof("Node %s : %d\n", n.name, n.inputMap[msgID])
	for _, child := range n.children {
		child.dump(msgID, indent+"     ")
	}
}

// addInput records the number of inputs each each node
// has to expect and records them in inputMap and returns the
// number of decendents of this node (including the node itself).
func (n *Node) addInput(msgID string) int {
	descendents := 0
	n.mutex.Lock()
	if _, ok := n.inputMap[msgID]; !ok {
		descendents = 1
	}
	n.inputMap[msgID] += 1
	n.mutex.Unlock()

	// If we have visited this node before that means we have
	// visited it's children and we can stop now.
	if descendents == 0 {
		return descendents
	}

	for _, child := range n.children {
		descendents += child.addInput(msgID)
	}
	return descendents
}

// receiver is the core processing function of this node that
// processes 'call' messages. When all inputs of a call are
// received it will trigger the function and pass the call
// message to the children of this node.
func (n *Node) receiver() {
	for {
		msg := <-n.inputCh
		// Check if the we have all inputs for this node.
		n.mutex.Lock()
		remaining := n.inputMap[msg.id]
		if remaining == 1 {
			delete(n.inputMap, msg.id)
		} else {
			n.inputMap[msg.id]--
		}
		n.mutex.Unlock()

		if remaining != 1 {
			continue
		}

		// We have all inputs now call the function for this node
		// and afterwards feed it to all the children.
		go func(msg *call) {
			// If there was an error. Skip the function call.
			if !msg.hasErr() {
				if err := n.procFn(msg.state); err != nil {
					msg.setErr(err)
				}
			}
			msg.wg.Done()

			// Feed into all the children asynchronously.
			for _, child := range n.children {
				go func(child *Node) {
					child.inputCh <- msg
				}(child)
			}
		}(msg)
	}
}

// call is the message type that is passed between the nodes
// of the DAG.
type call struct {
	id    string
	state interface{}
	errCh chan error
	wg    sync.WaitGroup
}

// hasErr returns true if a error has been set of this call.
func (c *call) hasErr() bool {
	return len(c.errCh) > 0
}

// setErr sets the error on this call. If an error has already been
// set, it logs an error message.
func (c *call) setErr(err error) {
	// If the error channel is not ready to receive it means an error
	// has already been set.
	select {
	case c.errCh <- err:
	default:
		sklog.Errorf("Error channel already set on call. Error: %s", err)
	}
}
