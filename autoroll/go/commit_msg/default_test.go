package commit_msg

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestNamedTemplateDefault_AllFeatures(t *testing.T) {
	unittest.SmallTest(t)

	b := fakeBuilder(t)
	result, err := b.Build(FakeCommitMsgInputs())
	require.NoError(t, err)
	require.Equal(t, `Roll fake/child/src from aaaaaaaaaaaa to cccccccccccc (2 revisions)

https://fake-child-log/aaaaaaaaaaaa..cccccccccccc

2020-04-16 c@google.com Commit C
2020-04-15 b@google.com Commit B

Also rolling transitive DEPS:
  parent/dep: dddddddddddd..eeeeeeeeeeee

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
https://fake.server.com/r/fake-autoroll
Please CC reviewer@google.com on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+/master/autoroll/README.md

Cq-Include-Trybots: some-trybot
Bug: fakebugproject:1234,fakebugproject:5678
Tbr: reviewer@google.com
Test: some-test
`, result)
}

func TestNamedTemplateDefault_NoLog(t *testing.T) {
	unittest.SmallTest(t)

	b := fakeBuilder(t)
	b.cfg.IncludeLog = false
	result, err := b.Build(FakeCommitMsgInputs())
	require.NoError(t, err)
	require.Equal(t, `Roll fake/child/src from aaaaaaaaaaaa to cccccccccccc (2 revisions)

https://fake-child-log/aaaaaaaaaaaa..cccccccccccc

Also rolling transitive DEPS:
  parent/dep: dddddddddddd..eeeeeeeeeeee

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
https://fake.server.com/r/fake-autoroll
Please CC reviewer@google.com on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+/master/autoroll/README.md

Cq-Include-Trybots: some-trybot
Bug: fakebugproject:1234,fakebugproject:5678
Tbr: reviewer@google.com
Test: some-test
`, result)
}

func TestNamedTemplateDefault_NoBugs(t *testing.T) {
	unittest.SmallTest(t)

	b := fakeBuilder(t)
	b.cfg.IncludeBugs = false
	result, err := b.Build(FakeCommitMsgInputs())
	require.NoError(t, err)
	require.Equal(t, `Roll fake/child/src from aaaaaaaaaaaa to cccccccccccc (2 revisions)

https://fake-child-log/aaaaaaaaaaaa..cccccccccccc

2020-04-16 c@google.com Commit C
2020-04-15 b@google.com Commit B

Also rolling transitive DEPS:
  parent/dep: dddddddddddd..eeeeeeeeeeee

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
https://fake.server.com/r/fake-autoroll
Please CC reviewer@google.com on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+/master/autoroll/README.md

Cq-Include-Trybots: some-trybot
Bug: None
Tbr: reviewer@google.com
Test: some-test
`, result)
}

func TestNamedTemplateDefault_Minimal(t *testing.T) {
	unittest.SmallTest(t)

	b := fakeBuilder(t)
	b.cfg.ChildLogURLTmpl = ""
	b.cfg.CqExtraTrybots = nil
	b.cfg.IncludeBugs = false
	b.cfg.IncludeLog = false
	b.cfg.IncludeTbrLine = false
	b.cfg.IncludeTests = false
	b.transitiveDeps = nil
	result, err := b.Build(FakeCommitMsgInputs())
	require.NoError(t, err)
	require.Equal(t, `Roll fake/child/src from aaaaaaaaaaaa to cccccccccccc (2 revisions)

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
https://fake.server.com/r/fake-autoroll
Please CC reviewer@google.com on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+/master/autoroll/README.md

Bug: None
`, result)
}
