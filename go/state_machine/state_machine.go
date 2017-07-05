package state_machine

/*
   Simple state machine implementation.
*/

import (
	"fmt"
	"sync"
)

type State string

type NextFn func() (State, error)
type Transition func() error

// Simple state machine implementation. At each tick, runs the transition
// function for the current state which determines the next state.
type StateMachine struct {
	current     State
	err         error
	mtx         sync.Mutex
	states      map[State]NextFn
	transitions map[State]map[State]Transition
}

// Return a StateMachine instance with the given default state.
func New(defaultState State) *StateMachine {
	return &StateMachine{
		current:     defaultState,
		err:         nil,
		mtx:         sync.Mutex{},
		states:      map[State]NextFn{},
		transitions: map[State]map[State]Transition{},
	}
}

// Add a new state, along with a function to run at that state to determine the
// next state.
func (s *StateMachine) AddState(state State, next NextFn) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.states[state] = next
}

// Add function to run when transitioning from the given starting State to the
// given ending State. Duplicate transitions are overridden.
func (s *StateMachine) AddTransition(from, to State, fn Transition) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	t, ok := s.transitions[from]
	if !ok {
		t = map[State]Transition{}
		s.transitions[from] = t
	}
	t[to] = fn
}

// Return the current state.
func (s *StateMachine) Current() State {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.current
}

// Get the next State.
func (s *StateMachine) GetNext() (State, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	fn, ok := s.states[s.current]
	if !ok {
		return "", fmt.Errorf("State %q not defined.", s.current)
	}
	return fn()
}

// Run the state transition function for the current state.
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
			return err
		}
	}
	s.current = dest
	return nil
}

// Get the next state and perform the transition.
func (s *StateMachine) NextTransition() error {
	next, err := s.GetNext()
	if err != nil {
		return fmt.Errorf("Failed to get next state: %s", err)
	}
	if err := s.Transition(next); err != nil {
		return fmt.Errorf("Failed to transition to state %q: %s", next, err)
	}
	return nil
}
