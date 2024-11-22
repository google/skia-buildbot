package exec

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/require"
)

// Adding TestMain allows us to have tests actually run a subprocess while being
// able to cleanly control the execution and output.
func TestMain(m *testing.M) {
	retrySwitch := os.Getenv("TEST_RETRY")
	switch retrySwitch {
	case "":
		os.Exit(m.Run())
	case "success":
		fmt.Println("ok")
		os.Exit(0)
	default:
		fmt.Println("not ok")
		os.Exit(1)
	}
}

func TestRetryContext(t *testing.T) {
	attempts := 0
	ctx := NewContext(context.Background(), func(ctx context.Context, cmd *Command) error {
		attempts++
		if attempts >= 3 {
			cmd.Env = []string{"TEST_RETRY=success"}
		} else {
			cmd.Env = []string{"TEST_RETRY=failure"}
		}
		return DefaultRun(ctx, cmd)
	})
	ctx = WithRetryContext(ctx, &backoff.ZeroBackOff{})
	thisBinary, err := os.Executable()
	require.NoError(t, err)
	output, err := RunCwd(ctx, ".", thisBinary)
	require.NoError(t, err)
	// Ensure that we collected output for all of the executions of the command.
	require.Equal(t, "not ok\nnot ok\nok\n", output)
}
