package util

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
)

// NamedErrGroup is like errgroup.Group, except each function in the group gets
// a name. It waits for all goroutines to finish and reports all errors by name.
type NamedErrGroup struct {
	errs map[string]error
	mtx  sync.Mutex
	wg   sync.WaitGroup
}

// NewNamedErrGroup returns a NamedErrGroup instance.
func NewNamedErrGroup() *NamedErrGroup {
	return &NamedErrGroup{
		errs: map[string]error{},
		mtx:  sync.Mutex{},
		wg:   sync.WaitGroup{},
	}
}

// Go runs the given function in a goroutine.
func (g *NamedErrGroup) Go(name string, fn func() error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if err := fn(); err != nil {
			g.mtx.Lock()
			defer g.mtx.Unlock()
			g.errs[name] = err
		}
	}()
}

// Wait waits for all of the goroutines to finish and reports any errors.
func (g *NamedErrGroup) Wait() error {
	g.wg.Wait()
	if len(g.errs) == 0 {
		return nil
	}
	msg := bytes.NewBufferString("NamedErrGroup encountered errors:\n")
	for name, err := range g.errs {
		e := fmt.Sprintf("\t%s: %s\n", name, err)
		_, _ = msg.Write([]byte(e))
	}
	return errors.New(string(msg.Bytes()))
}
