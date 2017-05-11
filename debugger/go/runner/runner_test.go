package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/skexec"
	"go.skia.org/infra/go/testutils"
)

func mockGetCurrentHash() string {
	return "aabbccdd"
}

// execString is the command line that would have been run through skexec.
var execString string

// testRun is a 'skexec.Run' function to use for testing.
func testRun(cmd *skexec.Command) error {
	execString = skexec.DebugString(cmd)
	return nil
}

func TestRunContainer(t *testing.T) {
	testutils.SmallTest(t)
	// Now test local runs, first set up exec for testing.
	exec.SetRun(testRun)
	defer exec.Reset()

	runner := New("/mnt/pd0/debugger", "/mnt/pd0/container", mockGetCurrentHash, false)
	err := runner.Start(20003)
	assert.NoError(t, err)
	assert.NotNil(t, runner)
	assert.Equal(t, "sudo systemd-nspawn -D /mnt/pd0/container --read-only --machine debug20003 --bind-ro /mnt/pd0/debugger xargs --arg-file=/dev/null /mnt/pd0/debugger/versions/aabbccdd/skia/out/Release/skiaserve --port 20003 --hosted", execString)
}

func TestRunLocal(t *testing.T) {
	testutils.SmallTest(t)
	// Now test local runs, first set up exec for testing.
	exec.SetRun(testRun)
	defer exec.Reset()

	runner := New("/mnt/pd0/debugger", "/mnt/pd0/container", mockGetCurrentHash, true)
	err := runner.Start(20003)
	assert.NoError(t, err)
	assert.NotNil(t, runner)
	assert.Equal(t, "/mnt/pd0/debugger/versions/aabbccdd/skia/out/Release/skiaserve --port 20003 --source  --hosted", execString)
}
