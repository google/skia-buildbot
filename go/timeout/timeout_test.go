package timeout

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTimeout(t *testing.T) {

	sleep := func(t time.Duration) func() error {
		return func() error {
			time.Sleep(t)
			return nil
		}
	}
	require.Equal(t, ErrTimedOut, Run(sleep(50*time.Millisecond), 10*time.Millisecond))
	require.Equal(t, nil, Run(sleep(10*time.Millisecond), 50*time.Millisecond))
	err := Run(func() error {
		return fmt.Errorf("blah")
	}, 100*time.Second)
	require.NotNil(t, err)
	require.Equal(t, "blah", err.Error())
}
