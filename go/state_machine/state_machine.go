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

type NextFn func() State

type TransitionFn func() error

// Simple state machine implementation. Each state defines a function which
// determines which state should come next. Generally these should contain logic
// and no actions. Transitions between states are defined using functions (or
// nil). Generally these should contain actions but no logic.
type StateMachine struct {
	current     State
	mtx         sync.Mutex
	states      map[State]NextFn
	transitions map[State]map[State]TransitionFn
}

// Return the current state.
func (s *StateMachine) Current() State {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.current
}

// Get the next State.
func (s *StateMachine) GetNext() State {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.states[s.current]()
}

// Run the state transition function from the current state to the given state.
func (s *StateMachine) Transition(dest State) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	t, ok := s.transitions[s.current]
	if !ok {
		return fmt.Errorf("No transitions defined from state %q", s.current)
	}
	fn, ok := t[dest]
	if !ok {
		return fmt.Errorf("No transition defined from state %q to %q", s.current, dest)
	}
	if fn != nil {
		if err := fn(); err != nil {
			return fmt.Errorf("Failed to transition to state %q: %s", dest, err)
		}
	}
	s.current = dest
	return nil
}

// Get the next state and perform the transition.
func (s *StateMachine) NextTransition() error {
	next := s.GetNext()
	return s.Transition(next)
}

// Tool for building StateMachines.
type StateMachineBuilder struct {
	initialState State
	states       map[State]NextFn
	transitions  map[State]map[State]TransitionFn
}

// Return a StateMachineBuilder instance.
func NewBuilder() *StateMachineBuilder {
	return &StateMachineBuilder{
		initialState: "",
		states:       map[State]NextFn{},
		transitions:  map[State]map[State]TransitionFn{},
	}
}

// SetInitial sets the initial State. Returns an error if the state has not been
// defined.
func (s *StateMachineBuilder) SetInitial(state State) error {
	if _, ok := s.states[state]; !ok {
		return fmt.Errorf("Undefined state %q", state)
	}
	s.initialState = state
	return nil
}

// Add a new State, along with a function to run at that State to determine the
// next state. Overrides any previous definition of this State.
func (s *StateMachineBuilder) AddState(state State, next NextFn) {
	s.states[state] = next
}

// Add function to run when transitioning from the given starting State to the
// given ending State. Duplicate transitions are overridden.
func (s *StateMachineBuilder) AddTransition(from, to State, fn TransitionFn) {
	t, ok := s.transitions[from]
	if !ok {
		t = map[State]TransitionFn{}
		s.transitions[from] = t
	}
	t[to] = fn
}

// Build a StateMachine instance. Returns an error if any transitions refer to
// states which have not been defined.
func (s *StateMachineBuilder) Build() (*StateMachine, error) {
	for from, toDict := range s.transitions {
		if _, ok := s.states[from]; !ok {
			return nil, fmt.Errorf("Transition from state %q but state %q is not defined!", from, from)
		}
		for to, _ := range toDict {
			if _, ok := s.states[to]; !ok {
				return nil, fmt.Errorf("Transition from state %q to state %q but state %q is not defined!", from, to, to)
			}
		}
	}
	if _, ok := s.states[s.initialState]; !ok {
		return nil, fmt.Errorf("Initial state %q is not defined!", s.initialState)
	}
	// TODO(borenet): Check for unreachable and unescapable states?
	return &StateMachine{
		current:     s.initialState,
		states:      s.states,
		transitions: s.transitions,
	}, nil
}

// PersistentStateMachine is a wrapper for StateMachine which persists its
// current state to a file.
type PersistentStateMachine struct {
	*StateMachine
	file string
}

// NewPersistentStateMachine returns a PersistentStateMachine using the given
// StateMachineBuilder. The initial state defined in the builder is overridden
// by the value in the persistent file, if it exists.
func NewPersistentStateMachine(file string, b *StateMachineBuilder) (*PersistentStateMachine, error) {
	initial := b.initialState
	contents, err := ioutil.ReadFile(file)
	if err == nil {
		initial = State(string(contents))
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("Unable to read file for PersistentStateMachine: %s", err)
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
	return &PersistentStateMachine{
		StateMachine: sm,
		file:         file,
	}, nil
}

// Run the state transition function from the current state to the given state.
func (s *PersistentStateMachine) Transition(dest State) error {
	if err := s.StateMachine.Transition(dest); err != nil {
		return err
	}
	return ioutil.WriteFile(s.file, []byte(s.Current()), os.ModePerm)
}

// Get the next state and perform the transition.
func (s *PersistentStateMachine) NextTransition() error {
	next := s.GetNext()
	return s.Transition(next)
}
