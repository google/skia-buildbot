// This file contains helper functions for TestCallStack in skerr_test.go. Modifying this file may
// cause the test assertions to be incorrect.

package alpha_test

import "go.skia.org/infra/go/skerr"

type Alpha struct {
	callback func() error
}

func (a *Alpha) SetWrappedCallback(callback func() error) {
	a.callback = func() error {
		if err := callback(); err != nil {
			return skerr.Wrap(err)
		} else {
			return nil
		}
	}
}

func (a *Alpha) Call() error {
	return a.callback()
}
