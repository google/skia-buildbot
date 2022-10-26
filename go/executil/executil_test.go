// We intentionally use the _test package here so that the tests import executil like client code
// would, making this demo of the API more realistic.
package executil_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/executil"
)

// runSomething would normally be in the code under test, where it has to invoke a command.
func runSomething(ctx context.Context) (string, error) {
	// Notice how the only functional difference in the code that uses the context is a call to
	// executil.CommandContext instead of os/exec.CommandContext.
	cmd := executil.CommandContext(ctx, "cowsay", "moo", "moooo")
	b, err := cmd.CombinedOutput()
	return string(b), err
}

func TestFakeTestsContext_SingleFakeTest_Success(t *testing.T) {
	ctx := executil.FakeTestsContext("Test_FakeExe_Cowsay_ReturnsASCIIArt")

	out, err := runSomething(ctx)
	// normally, require.NoError is what I would do here, but doing so would mask the outputs of
	// asserts made in the faked executable (which show up in the combined stdout/stderr.
	assert.NoError(t, err)
	assert.Equal(t, asciiArt, out)
}

func TestFakeTestsContext_SingleFakeTest_ReturnsErrorIfWrongArgumentsPassed(t *testing.T) {
	ctx := executil.FakeTestsContext("Test_FakeExe_Cowsay_ReturnsASCIIArt")

	cmd := executil.CommandContext(ctx, "cowsay", "wrong arguments")
	_, err := cmd.CombinedOutput()
	require.Error(t, err)
}

func TestFakeTestsContext_SingleFakeTest_CowsayFails_ReturnsError(t *testing.T) {
	ctx := executil.FakeTestsContext("Test_FakeExe_Cowsay_Crashes")

	out, err := runSomething(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "2") // exited code 2
	assert.Contains(t, out, "moo")       // some of the art was posted before it crashed
}

func TestFakeTestsContext_MultipleFakeTests_FirstSucceedsSecondReturnsError(t *testing.T) {
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_Cowsay_ReturnsASCIIArt", // should be run first
		"Test_FakeExe_Cowsay_Crashes")         // should be run second

	out, err := runSomething(ctx)
	assert.NoError(t, err)
	assert.Contains(t, out, asciiArt)

	_, err = runSomething(ctx)
	require.Error(t, err)

	assert.Equal(t, 2, executil.FakeCommandsReturned(ctx))
}

func TestWithFakeTests_ParentContextTimeoutRespected(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ctx = executil.WithFakeTests(ctx, "Test_FakeExe_Cowsay_Hangs")

	// This is an error because the context timed out. On Linux, this error is "signal: killed".
	_, err := runSomething(ctx)
	require.Error(t, err)
}

// This is not a real test, but a fake implementation of the executable in question (i.e. cowsay).
// By convention, we prefix these with FakeExe to make that clear.
func Test_FakeExe_Cowsay_ReturnsASCIIArt(t *testing.T) {
	// Since this is a normal go test, it will get run on the usual test suite. We check for the
	// special environment variable and if it is not set, we do nothing.
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"cowsay", "moo", "moooo"}, args)

	fmt.Printf(asciiArt)
	os.Exit(0) // exit 0 prevents golang from outputting test stuff like "=== RUN", "---Fail".
}

func Test_FakeExe_Cowsay_Crashes(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	args := executil.OriginalArgs()
	require.Equal(t, []string{"cowsay", "moo", "moooo"}, args)

	fmt.Println(asciiArt[:20])
	os.Exit(2)
}

func Test_FakeExe_Cowsay_Hangs(t *testing.T) {
	if executil.IsCallingFakeCommand() {
		// block forever. Hopefully this is called with a timeout.
		select {}
	}
}

const asciiArt = ` ___________
< moo moooo >
 -----------
        \   ^__^
         \  (oo)\_______
            (__)\       )\/\
                ||----w |
                ||     ||
`
