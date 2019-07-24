package state_machine

/*
  Simple state machine implementation.
*/

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/gcs"
)

// TransitionFn is a function to run when attempting to transition from one
// State to another. It is okay to give nil as a noop TransitionFn.
type TransitionFn func(ctx context.Context) error

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
func (b *Builder) F(name string, fn TransitionFn) {
	b.funcs[name] = func(ctx context.Context) error {
		if fn != nil {
			return fn(ctx)
		}
		return nil
	}
}

// Set the initial state.
func (b *Builder) SetInitial(s string) {
	b.initialState = s
}

// Build and return a StateMachine instance.
func (b *Builder) Build(ctx context.Context, gcsClient gcs.GCSClient, gsPath string) (*StateMachine, error) {
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
	cachedState := b.initialState
	contents, err := gcsClient.GetFileContents(ctx, gsPath)
	if err == nil {
		cachedState = string(contents)
	} else if err != storage.ErrObjectNotExist {
		return nil, fmt.Errorf("Unable to read file for persistentStateMachine: %s", err)
	}
	if _, ok := states[b.initialState]; !ok {
		return nil, fmt.Errorf("Initial state %q is not defined!", b.initialState)
	}

	// Every state must have transitions defined to and from it, even if
	// they are just self-transitions.
	for state := range states {
		if _, ok := transitions[state]; !ok {
			return nil, fmt.Errorf("No transitions defined from state %q", state)
		}
	}
	for _, toMap := range transitions {
		for to := range toMap {
			delete(states, to)
		}
	}
	for s := range states {
		if s != b.initialState {
			return nil, fmt.Errorf("No transitions defined to state %q", s)
		}
	}

	// Create and return the StateMachine.
	sm := &StateMachine{
		current:     cachedState,
		gcs:         gcsClient,
		funcs:       b.funcs,
		transitions: transitions,
		file:        gsPath,
	}

	// Write initial state back to GCS, in case it wasn't there before.
	if err := sm.gcs.SetFileContents(ctx, sm.file, gcs.FILE_WRITE_OPTS_TEXT, []byte(sm.Current())); err != nil {
		return nil, err
	}
	return sm, nil
}

// StateMachine is a simple state machine implementation which persists its
// current state to a file.
type StateMachine struct {
	current     string
	gcs         gcs.GCSClient
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
func (sm *StateMachine) Transition(ctx context.Context, dest string) error {
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

	// Run the transition func.
	if err := fn(ctx); err != nil {
		return fmt.Errorf("Failed to transition from %q to %q: %s", sm.current, dest, err)
	}
	sm.current = dest
	return sm.gcs.SetFileContents(ctx, sm.file, gcs.FILE_WRITE_OPTS_TEXT, []byte(sm.current))
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

// ListStates returns the list of all states in the StateMachine.
func (sm *StateMachine) ListStates() []string {
	sm.mtx.RLock()
	defer sm.mtx.RUnlock()
	// Every state must have at least one transition defined from it, so we
	// can use the keys in the outer transitions map to list the states.
	rv := make([]string, 0, len(sm.transitions))
	for state := range sm.transitions {
		rv = append(rv, state)
	}
	sort.Strings(rv)
	return rv
}
