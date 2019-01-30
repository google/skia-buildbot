package litevcs

import (
	"testing"

	"go.skia.org/infra/go/testutils"
	vcstu "go.skia.org/infra/go/vcsinfo/testutils"
)

func TestDisplay(t *testing.T) {
	testutils.LargeTest(t)
	vcs, cleanup := setupAndLoadRepo(t, skiaRepoURL, skiaRepoDir, true)
	defer cleanup()

	vcstu.TestDisplay(t, vcs)
}

func TestFrom(t *testing.T) {
	testutils.LargeTest(t)
	vcs, cleanup := setupAndLoadRepo(t, skiaRepoURL, skiaRepoDir, true)
	defer cleanup()

	vcstu.TestFrom(t, vcs)
}

func TestByIndex(t *testing.T) {
	testutils.LargeTest(t)
	vcs, cleanup := setupAndLoadRepo(t, skiaRepoURL, skiaRepoDir, true)
	defer cleanup()

	vcstu.TestByIndex(t, vcs)
}

func TestLastNIndex(t *testing.T) {
	testutils.LargeTest(t)
	vcs, cleanup := setupAndLoadRepo(t, skiaRepoURL, skiaRepoDir, true)
	defer cleanup()

	vcstu.TestLastNIndex(t, vcs)
}

func TestRange(t *testing.T) {
	testutils.LargeTest(t)
	vcs, cleanup := setupAndLoadRepo(t, skiaRepoURL, skiaRepoDir, true)
	defer cleanup()

	vcstu.TestRange(t, vcs)
}
