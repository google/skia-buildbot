package litevcs

import (
	"testing"

	"go.skia.org/infra/go/testutils"
	vcstu "go.skia.org/infra/go/vcsinfo/testutils"
)

func TestVCSSuite(t *testing.T) {
	testutils.LargeTest(t)
	vcs, cleanup := setupVCSLocalRepo(t)
	defer cleanup()

	// Run the VCS test suite.
	vcstu.TestDisplay(t, vcs)
	vcstu.TestFrom(t, vcs)
	vcstu.TestByIndex(t, vcs)
	vcstu.TestLastNIndex(t, vcs)
	vcstu.TestRange(t, vcs)
}
