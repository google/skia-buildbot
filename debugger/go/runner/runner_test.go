package runner

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/testutils"
)

// execString is the command line that would have been run through exec.
var execString string

// testRun is a 'exec.Run' function to use for testing.
func testRun(cmd *exec.Command) error {
	execString = exec.DebugString(cmd)
	return nil
}

func TestRunContainer(t *testing.T) {
	testutils.SmallTest(t)
	// Now test local runs, first set up exec for testing.
	ctx := exec.NewContext(context.Background(), testRun)
	runner := New()
	err := runner.Start(ctx, 20003)
	assert.NoError(t, err)
	assert.NotNil(t, runner)
	assert.Equal(t, "/bin/skiaserve --port 20003 --source  --hosted", execString)
}
