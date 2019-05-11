package timeout

import (
	"fmt"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestTimeout(t *testing.T) {
	unittest.LargeTest(t)

	sleep := func(t time.Duration) func() error {
		return func() error {
			time.Sleep(t)
			return nil
		}
	}
	assert.Equal(t, ErrTimedOut, Run(sleep(50*time.Millisecond), 10*time.Millisecond))
	assert.Equal(t, nil, Run(sleep(10*time.Millisecond), 50*time.Millisecond))
	err := Run(func() error {
		return fmt.Errorf("blah")
	}, 100*time.Second)
	assert.NotNil(t, err)
	assert.Equal(t, "blah", err.Error())
}
