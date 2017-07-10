package state_machine

/*
   Simple state machine implementation.
*/

import (
	"fmt"
	"io/ioutil"
	"os"
	"sync"
)

type State string

type TransitionFn func() error

type NextFn func() (State, TransitionFn)

// Simple state machine implementation. Each state defines a function which
// determines which state should come next. Generally these should contain logic
// and no actions. Transitions between states are returned by the aforementioned
// function (or nil, for no action). Generally these should contain actions but
// no logic.
type StateMachine interface {
	// Return the current state.
	Current() State

	// Attempt to transition to the next state.
	NextTransition() error
}

// Most basic implementation of StateMachine.
type stateMachine struct {
	current     State
	mtx         sync.Mutex
	states      map[State]NextFn
	transitions map[State]map[State]TransitionFn
}

// See documentation for StateMachine interface.
func (s *stateMachine) Current() State {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.current
}

// See documentation for StateMachine interface.
func (s *stateMachine) NextTransition() error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	next, transitionFn := s.states[s.current]()
	if transitionFn != nil {
		if err := transitionFn(); err != nil {
			return fmt.Errorf("Failed to transition to state %q: %s", next, err)
		}
	}
	s.current = next
	return nil
}

// Tool for building StateMachines.
type Builder struct {
	initialState State
	states       map[State]NextFn
	transitions  map[State]map[State]TransitionFn
}

// Return a Builder instance.
func NewBuilder() *Builder {
	return &Builder{
		initialState: "",
		states:       map[State]NextFn{},
		transitions:  map[State]map[State]TransitionFn{},
	}
}

// SetInitial sets the initial State.
func (b *Builder) SetInitial(state State) {
	b.initialState = state
}

// Add a new State, along with a function to run at that State to determine the
// next state. Overrides any previous definition of this State.
func (b *Builder) AddState(state State, next NextFn) {
	b.states[state] = next
}

// Validate returns an error if the state machine is not valid.
func (b *Builder) Validate() error {
	if _, ok := b.states[b.initialState]; !ok {
		return fmt.Errorf("Initial state %q is not defined!", b.initialState)
	}
	// TODO(borenet): Check for unreachable and unescapable states?
	return nil
}

// Build a StateMachine instance. Returns an error if the state machine is
// invalid.
func (b *Builder) Build() (StateMachine, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	return &stateMachine{
		current: b.initialState,
		states:  b.states,
	}, nil
}

// Build a persistent StateMachine instance. Returns an error if the state
// machine is invalid. The initial state defined in the builder is overridden
// by the value in the persistent file, if it exists.
func (b *Builder) BuildPersistent(file string) (StateMachine, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	initial := b.initialState
	contents, err := ioutil.ReadFile(file)
	if err == nil {
		initial = State(string(contents))
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("Unable to read file for persistentStateMachine: %s", err)
	}
	b.initialState = initial
	sm, err := b.Build()
	if err != nil {
		return nil, err
	}
	// Write initial state back to file, in case it wasn't there before.
	if err := ioutil.WriteFile(file, []byte(sm.Current()), os.ModePerm); err != nil {
		return nil, err
	}
	return &persistentStateMachine{
		stateMachine: sm.(*stateMachine),
		file:         file,
	}, nil
}

// persistentStateMachine is a wrapper for stateMachine which persists its
// current state to a file.
type persistentStateMachine struct {
	*stateMachine
	file string
}

// See documentation for StateMachine interface.
func (s *persistentStateMachine) NextTransition() error {
	if err := s.stateMachine.NextTransition(); err != nil {
		return err
	}
	return ioutil.WriteFile(s.file, []byte(s.Current()), os.ModePerm)
}
