package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/executil"
)

var (
	zone = Zone{
		filename: "skfe/skia.org.zone",
		project:  "skia-public",
		zoneName: "skia-org",
	}
)

func Test_ApplyZoneFile_Success(t *testing.T) {
	ctx := executil.FakeTestsContext("Test_FakeExe_GCloud_Successful")

	err := applyZoneFile(ctx, zone, "the-temp-filename")
	require.NoError(t, err)
}

func Test_ApplyZoneFile_GCloudFails_ReturnsErrorThatIncludesOutput(t *testing.T) {
	ctx := executil.FakeTestsContext("Test_FakeExe_GCloud_Fails")

	err := applyZoneFile(ctx, zone, "the-temp-filename")
	require.Contains(t, err.Error(),
		"Failed to apply zone file")
}

// This is not a real test, but a fake implementation of the executable in question (gcloud).
// By convention, we prefix these with FakeExe to make that clear.
func Test_FakeExe_GCloud_Successful(t *testing.T) {
	// Since this is a normal go test, it will get run on the usual test suite. We check for the
	// special environment variable and if it is not set, we do nothing.
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"gcloud", "dns", "record-sets", "import", "--project", "skia-public", "--delete-all-existing", "--zone", "skia-org", "--zone-file-format", "the-temp-filename"}, args)

	os.Exit(0) // exit 0 prevents golang from outputting test stuff like "=== RUN", "---Fail".
}

// This is not a real test, but a fake implementation of the executable in question (gcloud).
// By convention, we prefix these with FakeExe to make that clear.
func Test_FakeExe_GCloud_Fails(t *testing.T) {
	// Since this is a normal go test, it will get run on the usual test suite. We check for the
	// special environment variable and if it is not set, we do nothing.
	if !executil.IsCallingFakeCommand() {
		return
	}

	fmt.Println("Failed to apply zone file.")
	os.Exit(2)
}
