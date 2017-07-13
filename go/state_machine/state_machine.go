package state_machine

/*
   Wrapper around github.com/looplab/fsm.FSM which adds persistence.
*/

import (
	"fmt"
	"io/ioutil"
	"os"
	"sync"
)

// TransitionFn is a function to run when attempting to transition from one
// State to another. It is okay to give nil as a noop TransitionFn.
type TransitionFn func() error

// Builder is a helper struct used for constructing StateMachines.
type Builder struct {
	funcs        map[string]TransitionFn
	initialState string
	transitions  map[string]map[string]string
}

// NewBuilder returns a Builder instance.
func NewBuilder() *Builder {
	return &Builder{
		funcs:        map[string]TransitionFn{},
		initialState: "",
		transitions:  map[string]map[string]string{},
	}
}

// Add a transition between the two states with the given named function.
func (b *Builder) T(from, to, fn string) {
	toMap, ok := b.transitions[from]
	if !ok {
		toMap = map[string]string{}
		b.transitions[from] = toMap
	}
	toMap[to] = fn
}

// Add a transition function with the given name. Allows nil functions for no-op
// transitions.
func (b *Builder) F(name string, fn func() error) {
	b.funcs[name] = func() error {
		if fn != nil {
			return fn()
		}
		return nil
	}
}

// Set the initial state.
func (b *Builder) SetInitial(s string) {
	b.initialState = s
}

// Build and return a StateMachine instance.
func (b *Builder) Build(file string) (*StateMachine, error) {
	// Build and validate.
	states := make(map[string]bool, len(b.transitions))
	for from, toMap := range b.transitions {
		states[from] = true
		for to, fName := range toMap {
			states[to] = true
			if _, ok := b.funcs[fName]; !ok {
				return nil, fmt.Errorf("Function %q not defined.", fName)
			}
		}
	}

	// Get the previous state (if any) from the file.
	contents, err := ioutil.ReadFile(file)
	if err == nil {
		b.initialState = string(contents)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("Unable to read file for persistentStateMachine: %s", err)
	}
	if _, ok := states[b.initialState]; !ok {
		return nil, fmt.Errorf("Initial state %q is not defined!", b.initialState)
	}

	// Every state must have transitions defined to and from it, even if
	// they are just self-transitions.
	for state, _ := range states {
		if _, ok := b.transitions[state]; !ok {
			return nil, fmt.Errorf("No transitions defined from state %q", state)
		}
	}
	for _, toMap := range b.transitions {
		for to, _ := range toMap {
			delete(states, to)
		}
	}
	for s, _ := range states {
		return nil, fmt.Errorf("No transitions defined to state %q", s)
	}

	// Create and return the StateMachine.
	sm := &StateMachine{
		current:     b.initialState,
		funcs:       b.funcs,
		transitions: b.transitions,
		file:        file,
	}
	// Write initial state back to file, in case it wasn't there before.
	if err := ioutil.WriteFile(file, []byte(sm.Current()), os.ModePerm); err != nil {
		return nil, err
	}
	return sm, nil
}

// StateMachine is a simple state machine implementation which persists its
// current state to a file.
type StateMachine struct {
	current     string
	funcs       map[string]TransitionFn
	transitions map[string]map[string]string
	file        string
	mtx         sync.RWMutex
}

// Return the current state.
func (sm *StateMachine) Current() string {
	sm.mtx.RLock()
	defer sm.mtx.RUnlock()
	return sm.current
}

// Attempt to transition to the given state, using the transition function.
func (sm *StateMachine) Transition(dest string) error {
	sm.mtx.Lock()
	defer sm.mtx.Unlock()
	toMap, ok := sm.transitions[sm.current]
	if !ok {
		return fmt.Errorf("No transitions defined from state %q", sm.current)
	}
	fName, ok := toMap[dest]
	if !ok {
		return fmt.Errorf("No transition defined from state %q to state %q", sm.current, dest)
	}
	fn, ok := sm.funcs[fName]
	if !ok {
		return fmt.Errorf("Undefined transition function %q", fName)
	}
	if err := fn(); err != nil {
		return fmt.Errorf("Failed to transition from %q to %q: %s", sm.current, dest, err)
	}
	sm.current = dest
	return ioutil.WriteFile(sm.file, []byte(sm.current), os.ModePerm)
}

// Return the name of the transition function from the current state to the
// given state.
func (sm *StateMachine) GetTransitionName(dest string) (string, error) {
	sm.mtx.RLock()
	defer sm.mtx.RUnlock()
	toMap, ok := sm.transitions[sm.current]
	if !ok {
		return "", fmt.Errorf("No transitions defined from state %q", sm.current)
	}
	fName, ok := toMap[dest]
	if !ok {
		return "", fmt.Errorf("No transition defined from state %q to state %q", sm.current, dest)
	}
	return fName, nil
}
