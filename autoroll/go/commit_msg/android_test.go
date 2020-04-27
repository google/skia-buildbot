package commit_msg

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestNamedTemplateAndroid_AllFeatures(t *testing.T) {
	unittest.SmallTest(t)

	c := fakeCommitMsgConfig(t)
	c.CommitMsgTmpl = TmplNameAndroid
	result, err := c.BuildCommitMsg(fakeCommitMsgInputs())
	require.NoError(t, err)
	require.Equal(t, `Roll fake/child/src aaaaaaaaaaaa..cccccccccccc (2 commits)

https://fake-child-log/aaaaaaaaaaaa..cccccccccccc

2020-04-16 c@google.com Commit C
2020-04-15 b@google.com Commit B

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
https://fake.server.com/r/fake-autoroll
Please CC reviewer@google.com on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+/master/autoroll/README.md

Test: Presubmit checks will test this change.
Exempt-From-Owner-Approval: The autoroll bot does not require owner approval.
Bug: fakebugproject:1234
Bug: fakebugproject:5678
Test: some-test
`, result)
}

func TestNamedTemplateAndroid_NoLog(t *testing.T) {
	unittest.SmallTest(t)

	c := fakeCommitMsgConfig(t)
	c.CommitMsgTmpl = TmplNameAndroid
	c.IncludeLog = false
	result, err := c.BuildCommitMsg(fakeCommitMsgInputs())
	require.NoError(t, err)
	require.Equal(t, `Roll fake/child/src aaaaaaaaaaaa..cccccccccccc (2 commits)

https://fake-child-log/aaaaaaaaaaaa..cccccccccccc

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
https://fake.server.com/r/fake-autoroll
Please CC reviewer@google.com on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+/master/autoroll/README.md

Test: Presubmit checks will test this change.
Exempt-From-Owner-Approval: The autoroll bot does not require owner approval.
Bug: fakebugproject:1234
Bug: fakebugproject:5678
Test: some-test
`, result)
}

func TestNamedTemplateAndroid_NoBugs(t *testing.T) {
	unittest.SmallTest(t)

	c := fakeCommitMsgConfig(t)
	c.CommitMsgTmpl = TmplNameAndroid
	c.IncludeBugs = false
	result, err := c.BuildCommitMsg(fakeCommitMsgInputs())
	require.NoError(t, err)
	require.Equal(t, `Roll fake/child/src aaaaaaaaaaaa..cccccccccccc (2 commits)

https://fake-child-log/aaaaaaaaaaaa..cccccccccccc

2020-04-16 c@google.com Commit C
2020-04-15 b@google.com Commit B

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
https://fake.server.com/r/fake-autoroll
Please CC reviewer@google.com on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+/master/autoroll/README.md

Test: Presubmit checks will test this change.
Exempt-From-Owner-Approval: The autoroll bot does not require owner approval.
Test: some-test
`, result)
}

func TestNamedTemplateAndroid_Minimal(t *testing.T) {
	unittest.SmallTest(t)

	c := fakeCommitMsgConfig(t)
	c.CommitMsgTmpl = TmplNameAndroid
	c.ChildLogURLTmpl = ""
	c.CqExtraTrybots = ""
	c.IncludeBugs = false
	c.IncludeLog = false
	c.IncludeTests = false
	c.TransitiveDeps = nil
	result, err := c.BuildCommitMsg(fakeCommitMsgInputs())
	require.NoError(t, err)
	require.Equal(t, `Roll fake/child/src aaaaaaaaaaaa..cccccccccccc (2 commits)

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
https://fake.server.com/r/fake-autoroll
Please CC reviewer@google.com on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+/master/autoroll/README.md

Test: Presubmit checks will test this change.
Exempt-From-Owner-Approval: The autoroll bot does not require owner approval.
`, result)
}
