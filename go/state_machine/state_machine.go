package state_machine

/*
  Simple state machine implementation.
*/

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sync"

	"go.skia.org/infra/go/sklog"
)

const (
	backingFile = "state_machine"
	busyFile    = "state_machine_transitioning"
)

// TransitionFn is a function to run when attempting to transition from one
// State to another. It is okay to give nil as a noop TransitionFn.
type TransitionFn func() error

type transition struct {
	from string
	to   string
	fn   string
}

// Builder is a helper struct used for constructing StateMachines.
type Builder struct {
	funcs        map[string]TransitionFn
	initialState string
	transitions  []transition
}

// NewBuilder returns a Builder instance.
func NewBuilder() *Builder {
	return &Builder{
		funcs:        map[string]TransitionFn{},
		initialState: "",
		transitions:  []transition{},
	}
}

// Add a transition between the two states with the given named function.
func (b *Builder) T(from, to, fn string) {
	b.transitions = append(b.transitions, transition{from: from, to: to, fn: fn})
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
func (b *Builder) Build(workdir string) (*StateMachine, error) {
	// Build and validate.
	transitions := map[string]map[string]string{}
	states := make(map[string]bool, len(b.transitions))
	for _, t := range b.transitions {
		states[t.from] = true
		states[t.to] = true
		toMap, ok := transitions[t.from]
		if !ok {
			toMap = map[string]string{}
			transitions[t.from] = toMap
		}
		if fn, ok := toMap[t.to]; ok {
			return nil, fmt.Errorf("Multiple defined transitions from %q to %q: %q, %q", t.from, t.to, fn, t.fn)
		}
		toMap[t.to] = t.fn
		if _, ok := b.funcs[t.fn]; !ok {
			return nil, fmt.Errorf("Function %q not defined.", t.fn)
		}
	}

	// Get the previous state (if any) from the file.
	file := path.Join(workdir, backingFile)
	cachedState := b.initialState
	contents, err := ioutil.ReadFile(file)
	if err == nil {
		cachedState = string(contents)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("Unable to read file for persistentStateMachine: %s", err)
	}
	if _, ok := states[b.initialState]; !ok {
		return nil, fmt.Errorf("Initial state %q is not defined!", b.initialState)
	}

	// Every state must have transitions defined to and from it, even if
	// they are just self-transitions.
	for state, _ := range states {
		if _, ok := transitions[state]; !ok {
			return nil, fmt.Errorf("No transitions defined from state %q", state)
		}
	}
	for _, toMap := range transitions {
		for to, _ := range toMap {
			delete(states, to)
		}
	}
	for s, _ := range states {
		if s != b.initialState {
			return nil, fmt.Errorf("No transitions defined to state %q", s)
		}
	}

	// Create and return the StateMachine.
	sm := &StateMachine{
		current:     cachedState,
		funcs:       b.funcs,
		transitions: transitions,
		file:        file,
		busyFile:    path.Join(workdir, busyFile),
	}

	// Check that we didn't interrupt a previous transition.
	if err := sm.checkBusy(); err != nil {
		return nil, err
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
	busyFile    string
	mtx         sync.RWMutex
}

// Return the current state.
func (sm *StateMachine) Current() string {
	sm.mtx.RLock()
	defer sm.mtx.RUnlock()
	return sm.current
}

// checkBusy returns an error if the "transitioning" file exists, indicating
// that a previous transition was interrupted.
func (sm *StateMachine) checkBusy() error {
	contents, err := ioutil.ReadFile(sm.busyFile)
	if err == nil {
		return fmt.Errorf("Transition to %q already in progress; did a previous transition get interrupted?", string(contents))
	}
	return nil
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

	// Write the busy file.
	if err := sm.checkBusy(); err != nil {
		return err
	}
	if err := ioutil.WriteFile(sm.busyFile, []byte(dest), os.ModePerm); err != nil {
		return err
	}
	defer func() {
		if err := os.Remove(sm.busyFile); err != nil {
			sklog.Errorf("Failed to remove busy file: %s", err)
		}
	}()

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
