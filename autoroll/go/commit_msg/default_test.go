package commit_msg

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestNamedTemplateDefault_AllFeatures(t *testing.T) {
	unittest.SmallTest(t)

	b := fakeBuilder(t)
	result, err := b.Build(FakeCommitMsgInputs())
	require.NoError(t, err)
	require.Equal(t, `Roll fake/child/src from aaaaaaaaaaaa to cccccccccccc (2 revisions)

https://fake-child-log/aaaaaaaaaaaa..cccccccccccc

2020-04-17 c@google.com Commit C
2020-04-16 b@google.com Commit B

Also rolling transitive DEPS:
  parent/dep1 from dddddddddddd to eeeeeeeeeeee
  parent/dep3 from aaaaaaaaaaaa to cccccccccccc

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
https://fake.server.com/r/fake-autoroll
Please CC reviewer@google.com on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+doc/main/autoroll/README.md

Cq-Include-Trybots: some-trybot-on-m92
Cq-Do-Not-Cancel-Tryjobs: true
Bug: fakebugproject:1234,fakebugproject:5678
Tbr: reviewer@google.com
Test: some-test
My-Footer: BlahBlah
My-Other-Footer: Blah
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
  parent/dep1 from dddddddddddd to eeeeeeeeeeee
  parent/dep3 from aaaaaaaaaaaa to cccccccccccc

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
https://fake.server.com/r/fake-autoroll
Please CC reviewer@google.com on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+doc/main/autoroll/README.md

Cq-Include-Trybots: some-trybot-on-m92
Cq-Do-Not-Cancel-Tryjobs: true
Bug: fakebugproject:1234,fakebugproject:5678
Tbr: reviewer@google.com
Test: some-test
My-Footer: BlahBlah
My-Other-Footer: Blah
`, result)
}

func TestNamedTemplateDefault_NoBugProject(t *testing.T) {
	unittest.SmallTest(t)

	b := fakeBuilder(t)
	b.cfg.BugProject = ""
	result, err := b.Build(FakeCommitMsgInputs())
	require.NoError(t, err)
	require.Equal(t, `Roll fake/child/src from aaaaaaaaaaaa to cccccccccccc (2 revisions)

https://fake-child-log/aaaaaaaaaaaa..cccccccccccc

2020-04-17 c@google.com Commit C
2020-04-16 b@google.com Commit B

Also rolling transitive DEPS:
  parent/dep1 from dddddddddddd to eeeeeeeeeeee
  parent/dep3 from aaaaaaaaaaaa to cccccccccccc

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
https://fake.server.com/r/fake-autoroll
Please CC reviewer@google.com on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+doc/main/autoroll/README.md

Cq-Include-Trybots: some-trybot-on-m92
Cq-Do-Not-Cancel-Tryjobs: true
Tbr: reviewer@google.com
Test: some-test
My-Footer: BlahBlah
My-Other-Footer: Blah
`, result)
}

func TestNamedTemplateDefault_NoBugs(t *testing.T) {
	unittest.SmallTest(t)

	b := fakeBuilder(t)
	from, to, revs, emails, canary := FakeCommitMsgInputs()
	from.Bugs = nil
	to.Bugs = nil
	for _, rev := range revs {
		rev.Bugs = nil
	}
	result, err := b.Build(from, to, revs, emails, canary)
	require.NoError(t, err)
	require.Equal(t, `Roll fake/child/src from aaaaaaaaaaaa to cccccccccccc (2 revisions)

https://fake-child-log/aaaaaaaaaaaa..cccccccccccc

2020-04-17 c@google.com Commit C
2020-04-16 b@google.com Commit B

Also rolling transitive DEPS:
  parent/dep1 from dddddddddddd to eeeeeeeeeeee
  parent/dep3 from aaaaaaaaaaaa to cccccccccccc

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
https://fake.server.com/r/fake-autoroll
Please CC reviewer@google.com on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+doc/main/autoroll/README.md

Cq-Include-Trybots: some-trybot-on-m92
Cq-Do-Not-Cancel-Tryjobs: true
Bug: None
Tbr: reviewer@google.com
Test: some-test
My-Footer: BlahBlah
My-Other-Footer: Blah
`, result)
}

func TestNamedTemplateDefault_Minimal(t *testing.T) {
	unittest.SmallTest(t)

	b := fakeBuilder(t)
	b.cfg.BugProject = ""
	b.cfg.ChildLogUrlTmpl = ""
	b.cfg.CqExtraTrybots = nil
	b.cfg.CqDoNotCancelTrybots = false
	b.cfg.ExtraFooters = nil
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
https://skia.googlesource.com/buildbot/+doc/main/autoroll/README.md
`, result)
}

func TestTotalOverride(t *testing.T) {
	unittest.SmallTest(t)

	b := fakeBuilder(t)
	b.cfg.Template = &config.CommitMsgConfig_Custom{
		Custom: `{{ define "commitMsg" }}Completely custom commit message.

Seriously, this can be anything at all.

Variables from config_vars should work, eg. m{{.Branches.Chromium.Beta.Milestone}}, v8:{{.Branches.Chromium.Beta.V8Branch}}
{{end}}`,
	}
	result, err := b.Build(FakeCommitMsgInputs())
	require.NoError(t, err)
	require.Equal(t, `Completely custom commit message.

Seriously, this can be anything at all.

Variables from config_vars should work, eg. m92, v8:9.2-lkgr
`, result)
}

func TestNamedTemplateDefault_BugLinks(t *testing.T) {
	unittest.SmallTest(t)

	b := fakeBuilder(t)
	b.childBugLink = fakeChildBugLink
	b.parentBugLink = fakeParentBugLink
	result, err := b.Build(FakeCommitMsgInputs())
	require.NoError(t, err)
	require.Equal(t, `Roll fake/child/src from aaaaaaaaaaaa to cccccccccccc (2 revisions)

https://fake-child-log/aaaaaaaaaaaa..cccccccccccc

2020-04-17 c@google.com Commit C
2020-04-16 b@google.com Commit B

Also rolling transitive DEPS:
  parent/dep1 from dddddddddddd to eeeeeeeeeeee
  parent/dep3 from aaaaaaaaaaaa to cccccccccccc

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
https://fake.server.com/r/fake-autoroll
Please CC reviewer@google.com on the revert to ensure that a human
is aware of the problem.

To file a bug in fake/child/src: https://file-a-child-bug.com
To file a bug in fake/parent: https://file-a-parent-bug.com

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+doc/main/autoroll/README.md

Cq-Include-Trybots: some-trybot-on-m92
Cq-Do-Not-Cancel-Tryjobs: true
Bug: fakebugproject:1234,fakebugproject:5678
Tbr: reviewer@google.com
Test: some-test
My-Footer: BlahBlah
My-Other-Footer: Blah
`, result)
}
