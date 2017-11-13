package timeout

import (
	"fmt"
	"time"
)

var (
	ErrTimedOut = fmt.Errorf("Function call timed out.")
)

// Run the given function with the given timeout. Return any error returned by
// the function, or ErrTimedOut if the timeout took place. The function is not
// interrupted if the timeout takes place.
func Run(fn func() error, timeout time.Duration) error {
	c := make(chan error, 1)
	go func() {
		c <- fn()
		close(c)
	}()
	t := time.NewTimer(timeout)
	defer t.Stop()

	select {
	case err := <-c:
		return err
	case <-t.C:
		return ErrTimedOut
	}
}
