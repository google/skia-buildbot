package commit_msg

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestNamedTemplateAfdo_AllFeatures(t *testing.T) {
	unittest.SmallTest(t)

	c := fakeCommitMsgConfig(t)
	c.CommitMsgTmpl = TmplNameAfdo
	result, err := c.BuildCommitMsg(fakeCommitMsgInputs())
	require.NoError(t, err)
	require.Equal(t, `Roll AFDO from aaaaaaaaaaaa to cccccccccccc

This CL may cause a small binary size increase, roughly proportional
to how long it's been since our last AFDO profile roll. For larger
increases (around or exceeding 100KB), please file a bug against
gbiv@chromium.org. Additional context: https://crbug.com/805539

Please note that, despite rolling to chrome/android, this profile is
used for both Linux and Android.

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
https://fake.server.com/r/fake-autoroll
Please CC reviewer@google.com on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+/master/autoroll/README.md

Tbr: reviewer@google.com
`, result)
}
