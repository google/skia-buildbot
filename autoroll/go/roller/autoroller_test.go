package roller

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/roller_cleanup"
	"go.skia.org/infra/autoroll/go/roller_cleanup/mocks"
	"go.skia.org/infra/go/now"
)

func TestAutoRollerRolledPast(t *testing.T) {

	ctx := context.Background()
	r := &AutoRoller{}
	rev := func(id string) *revision.Revision {
		return &revision.Revision{Id: id}
	}
	r.lastRollRev = rev("0")
	r.nextRollRev = rev("1") // Pretend we're configured to roll one rev at a time.
	r.tipRev = rev("5")
	r.notRolledRevs = []*revision.Revision{
		rev("5"),
		rev("4"),
		rev("3"),
		rev("2"),
		rev("1"),
	}

	check := func(id string, expect bool) {
		got, err := r.RolledPast(ctx, &revision.Revision{Id: id})
		require.NoError(t, err)
		require.Equal(t, expect, got)
	}

	check("0", true)              // lastRollRev
	check("1", false)             // nextRollRev
	check("2", false)             // notRolledRev
	check("3", false)             // notRolledRev
	check("4", false)             // notRolledRev
	check("5", false)             // tipRev
	check("some other rev", true) // everything else
}

func TestDeleteCheckoutAndExit(t *testing.T) {
	// Create some files and directories to be deleted. Include both normal and
	// hidden files and dirs, with nested files.
	tmp := t.TempDir()
	dirs := []string{
		filepath.Join(tmp, ".hiddendir"),
		filepath.Join(tmp, "normaldir"),
	}
	files := []string{
		filepath.Join(tmp, ".hiddenfile"),
		filepath.Join(tmp, "normalfile"),
		filepath.Join(tmp, ".hiddendir", "nested"),
		filepath.Join(tmp, "normaldir", "nested"),
	}
	for _, dir := range dirs {
		require.NoError(t, os.MkdirAll(dir, os.ModePerm))
	}
	for _, file := range files {
		require.NoError(t, os.WriteFile(file, []byte("blahblah"), os.ModePerm))
	}

	// Create the roller.
	mockCleanup := &mocks.DB{}
	r := &AutoRoller{
		cleanup: mockCleanup,
		roller:  "my-roller",
		workdir: tmp,
	}
	ts := time.Unix(1715005596, 0) // Arbitrary timestamp.
	nowProvider := func() time.Time {
		return ts
	}
	ctx := context.WithValue(context.Background(), now.ContextKey, now.NowProvider(nowProvider))

	// Mock the request to clear the needs-cleanup bit.
	mockCleanup.On("RequestCleanup", ctx, &roller_cleanup.CleanupRequest{
		RollerID:      r.roller,
		NeedsCleanup:  false,
		User:          r.roller,
		Timestamp:     ts,
		Justification: "Deleted local data",
	}).Return(nil)

	// DeleteLocalData.
	require.NoError(t, r.DeleteLocalData(ctx))

	// Ensure that tmp still exists (for most rollers this is a mounted
	// directory which we cannot delete) but is empty.
	st, err := os.Stat(tmp)
	require.NoError(t, err)
	require.True(t, st.IsDir())

	// Use os.Stat for each of the listed files and directories rather than
	// os.ReadDir, just in case that doesn't return the hidden files and dirs.
	for _, dir := range dirs {
		_, err := os.Stat(dir)
		require.True(t, os.IsNotExist(err))
	}
	for _, file := range files {
		_, err := os.Stat(file)
		require.True(t, os.IsNotExist(err))
	}
}
