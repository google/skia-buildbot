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
type StateMachine interface {
	// Return the current state.
	Current() State

	// Return the next state.
	GetNext() State

	// Attempt to transition to the given state.
	Transition(State) error

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
func (s *stateMachine) GetNext() State {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.states[s.current]()
}

// See documentation for StateMachine interface.
func (s *stateMachine) Transition(dest State) error {
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

// See documentation for StateMachine interface.
func (s *stateMachine) NextTransition() error {
	next := s.GetNext()
	return s.Transition(next)
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

// Add function to run when transitioning from the given starting State to the
// given ending State. Duplicate transitions are overridden.
func (b *Builder) AddTransition(from, to State, fn TransitionFn) {
	t, ok := b.transitions[from]
	if !ok {
		t = map[State]TransitionFn{}
		b.transitions[from] = t
	}
	t[to] = fn
}

// Validate returns an error if the state machine is not valid.
func (b *Builder) Validate() error {
	for from, toDict := range b.transitions {
		if _, ok := b.states[from]; !ok {
			return fmt.Errorf("Transition from state %q but state %q is not defined!", from, from)
		}
		for to, _ := range toDict {
			if _, ok := b.states[to]; !ok {
				return fmt.Errorf("Transition from state %q to state %q but state %q is not defined!", from, to, to)
			}
		}
	}
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
		current:     b.initialState,
		states:      b.states,
		transitions: b.transitions,
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
func (s *persistentStateMachine) Transition(dest State) error {
	if err := s.stateMachine.Transition(dest); err != nil {
		return err
	}
	return ioutil.WriteFile(s.file, []byte(s.Current()), os.ModePerm)
}

// See documentation for StateMachine interface.
func (s *persistentStateMachine) NextTransition() error {
	next := s.GetNext()
	return s.Transition(next)
}
