package testutils

import (
	"sync"

	"github.com/stretchr/testify/mock"
)

// These helpers assist in wrapping the Run calls in the wait group
// so we can be sure everything actually runs before the function
// terminates. Ideally, I would have liked to be able to chain multiple
// Run calls to the mock, but testify's mocks only allow one
// Run per Call. We have two helpers then, one if a mock does not already
// have a Run function and the other is for wrapping around a Run function
// that already exists.
// Note: do not call defer wg.Wait() because if any assert fails, it will
// panic, possibly before all the wg.Done() are called, causing a deadlock.
func AsyncHelpers() (*sync.WaitGroup, func(c *mock.Call) *mock.Call, func(f func(args mock.Arguments)) func(mock.Arguments)) {
	wg := sync.WaitGroup{}
	isAsync := func(c *mock.Call) *mock.Call {
		wg.Add(1)
		return c.Run(func(a mock.Arguments) {
			wg.Done()
		})
	}
	asyncWrapper := func(f func(args mock.Arguments)) func(mock.Arguments) {
		wg.Add(1)
		return func(args mock.Arguments) {
			defer wg.Done()
			f(args)
		}
	}
	return &wg, isAsync, asyncWrapper
}
