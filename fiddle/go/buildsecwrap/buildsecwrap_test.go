package buildsecwrap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/testutils"
)

// execStrings are the command lines that would have been run through exec.
var execStrings []string = []string{}

// testRun is a 'exec.Run' function to use for testing.
func testRun(cmd *exec.Command) error {
	execStrings = append(execStrings, exec.DebugString(cmd))
	return nil
}

func TestRun(t *testing.T) {
	testutils.SmallTest(t)
	// Now test local runs, first set up exec for testing.
	exec.SetRunForTesting(testRun)
	defer exec.SetRunForTesting(exec.DefaultRun)

	err := Build("/tmp")
	assert.NoError(t, err)
	assert.Equal(t, "c++ /tmp/bin/fiddle_secwrap.cpp -o /tmp/bin/fiddle_secwrap", execStrings[0])
}
