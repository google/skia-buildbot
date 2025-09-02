package commit_msg

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNamedTemplateDefault_AllFeatures(t *testing.T) {

	b := fakeBuilder(t)
	result, err := b.Build(FakeCommitMsgInputs())
	require.NoError(t, err)
	require.Equal(t, `Roll fake/child/src from aaaaaaaaaaaa to cccccccccccc (2 revisions)

https://fake-child-log/aaaaaaaaaaaa..cccccccccccc

2020-04-17 c@google.com Commit C
2020-04-16 b@google.com Commit B

Also rolling transitive DEPS:
  https://fake-dep1/+log/dddddddddddddddddddddddddddddddddddddddd..eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee
  parent/dep3 from aaaaaaaaaaaa to cccccccccccc

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
https://fake.server.com/r/fake-autoroll
Please CC contact@google.com,reviewer@google.com on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://issues.skia.org/issues/new?component=1389291&template=1850622

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+doc/main/autoroll/README.md

Cq-Include-Trybots: luci.fakeproject.try:some-trybot-on-m92
Cq-Do-Not-Cancel-Tryjobs: true
Bug: fakebugproject:1234,fakebugproject:5678
Tbr: reviewer@google.com
Test: some-test
My-Footer: BlahBlah
My-Other-Footer: Blah
`, result)
}

func TestNamedTemplateDefault_NoLog(t *testing.T) {

	b := fakeBuilder(t)
	b.cfg.IncludeLog = false
	result, err := b.Build(FakeCommitMsgInputs())
	require.NoError(t, err)
	require.Equal(t, `Roll fake/child/src from aaaaaaaaaaaa to cccccccccccc (2 revisions)

https://fake-child-log/aaaaaaaaaaaa..cccccccccccc

Also rolling transitive DEPS:
  https://fake-dep1/+log/dddddddddddddddddddddddddddddddddddddddd..eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee
  parent/dep3 from aaaaaaaaaaaa to cccccccccccc

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
https://fake.server.com/r/fake-autoroll
Please CC contact@google.com,reviewer@google.com on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://issues.skia.org/issues/new?component=1389291&template=1850622

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+doc/main/autoroll/README.md

Cq-Include-Trybots: luci.fakeproject.try:some-trybot-on-m92
Cq-Do-Not-Cancel-Tryjobs: true
Bug: fakebugproject:1234,fakebugproject:5678
Tbr: reviewer@google.com
Test: some-test
My-Footer: BlahBlah
My-Other-Footer: Blah
`, result)
}

func TestNamedTemplateDefault_NoBugProject(t *testing.T) {

	b := fakeBuilder(t)
	b.cfg.BugProject = ""
	result, err := b.Build(FakeCommitMsgInputs())
	require.NoError(t, err)
	require.Equal(t, `Roll fake/child/src from aaaaaaaaaaaa to cccccccccccc (2 revisions)

https://fake-child-log/aaaaaaaaaaaa..cccccccccccc

2020-04-17 c@google.com Commit C
2020-04-16 b@google.com Commit B

Also rolling transitive DEPS:
  https://fake-dep1/+log/dddddddddddddddddddddddddddddddddddddddd..eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee
  parent/dep3 from aaaaaaaaaaaa to cccccccccccc

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
https://fake.server.com/r/fake-autoroll
Please CC contact@google.com,reviewer@google.com on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://issues.skia.org/issues/new?component=1389291&template=1850622

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+doc/main/autoroll/README.md

Cq-Include-Trybots: luci.fakeproject.try:some-trybot-on-m92
Cq-Do-Not-Cancel-Tryjobs: true
Tbr: reviewer@google.com
Test: some-test
My-Footer: BlahBlah
My-Other-Footer: Blah
`, result)
}

func TestNamedTemplateDefault_NoBugs(t *testing.T) {

	b := fakeBuilder(t)
	from, to, revs, emails, canary, contacts, manualRollRequester := FakeCommitMsgInputs()
	from.Bugs = nil
	to.Bugs = nil
	for _, rev := range revs {
		rev.Bugs = nil
	}
	result, err := b.Build(from, to, revs, emails, canary, contacts, manualRollRequester)
	require.NoError(t, err)
	require.Equal(t, `Roll fake/child/src from aaaaaaaaaaaa to cccccccccccc (2 revisions)

https://fake-child-log/aaaaaaaaaaaa..cccccccccccc

2020-04-17 c@google.com Commit C
2020-04-16 b@google.com Commit B

Also rolling transitive DEPS:
  https://fake-dep1/+log/dddddddddddddddddddddddddddddddddddddddd..eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee
  parent/dep3 from aaaaaaaaaaaa to cccccccccccc

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
https://fake.server.com/r/fake-autoroll
Please CC contact@google.com,reviewer@google.com on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://issues.skia.org/issues/new?component=1389291&template=1850622

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+doc/main/autoroll/README.md

Cq-Include-Trybots: luci.fakeproject.try:some-trybot-on-m92
Cq-Do-Not-Cancel-Tryjobs: true
Bug: None
Tbr: reviewer@google.com
Test: some-test
My-Footer: BlahBlah
My-Other-Footer: Blah
`, result)
}

func TestNamedTemplateDefault_Minimal(t *testing.T) {

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
Please CC contact@google.com,reviewer@google.com on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://issues.skia.org/issues/new?component=1389291&template=1850622

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+doc/main/autoroll/README.md
`, result)
}

func TestTotalOverride(t *testing.T) {

	b := fakeBuilder(t)
	b.cfg.Custom = `{{ define "commitMsg" }}Completely custom commit message.

Seriously, this can be anything at all.

Variables from config_vars should work, eg. m{{.Branches.Chromium.Beta.Milestone}}, v8:{{.Branches.Chromium.Beta.V8Branch}}-lkgr
{{end}}`
	result, err := b.Build(FakeCommitMsgInputs())
	require.NoError(t, err)
	require.Equal(t, `Completely custom commit message.

Seriously, this can be anything at all.

Variables from config_vars should work, eg. m92, v8:9.2-lkgr
`, result)
}

func TestNamedTemplateDefault_BugLinks(t *testing.T) {

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
  https://fake-dep1/+log/dddddddddddddddddddddddddddddddddddddddd..eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee
  parent/dep3 from aaaaaaaaaaaa to cccccccccccc

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
https://fake.server.com/r/fake-autoroll
Please CC contact@google.com,reviewer@google.com on the revert to ensure that a human
is aware of the problem.

To file a bug in fake/child/src: https://file-a-child-bug.com
To file a bug in fake/parent: https://file-a-parent-bug.com

To report a problem with the AutoRoller itself, please file a bug:
https://issues.skia.org/issues/new?component=1389291&template=1850622

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+doc/main/autoroll/README.md

Cq-Include-Trybots: luci.fakeproject.try:some-trybot-on-m92
Cq-Do-Not-Cancel-Tryjobs: true
Bug: fakebugproject:1234,fakebugproject:5678
Tbr: reviewer@google.com
Test: some-test
My-Footer: BlahBlah
My-Other-Footer: Blah
`, result)
}

func TestNamedTemplateDefault_ManualRoll(t *testing.T) {

	b := fakeBuilder(t)
	from, to, revs, emails, canary, contacts, manualRollRequester := FakeCommitMsgInputs()
	manualRollRequester = "manual-requester@google.com"
	result, err := b.Build(from, to, revs, emails, canary, contacts, manualRollRequester)
	require.NoError(t, err)
	require.Equal(t, `Manual roll fake/child/src from aaaaaaaaaaaa to cccccccccccc (2 revisions)

Manual roll requested by manual-requester@google.com

https://fake-child-log/aaaaaaaaaaaa..cccccccccccc

2020-04-17 c@google.com Commit C
2020-04-16 b@google.com Commit B

Also rolling transitive DEPS:
  https://fake-dep1/+log/dddddddddddddddddddddddddddddddddddddddd..eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee
  parent/dep3 from aaaaaaaaaaaa to cccccccccccc

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
https://fake.server.com/r/fake-autoroll
Please CC contact@google.com,reviewer@google.com on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://issues.skia.org/issues/new?component=1389291&template=1850622

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+doc/main/autoroll/README.md

Cq-Include-Trybots: luci.fakeproject.try:some-trybot-on-m92
Cq-Do-Not-Cancel-Tryjobs: true
Bug: fakebugproject:1234,fakebugproject:5678
Tbr: reviewer@google.com
Test: some-test
My-Footer: BlahBlah
My-Other-Footer: Blah
`, result)
}

func TestNamedTemplateDefault_WordWrap(t *testing.T) {
	b := fakeBuilder(t)
	b.wordWrapChars = 72
	from, to, revs, emails, canary, contacts, manualRollRequester := FakeCommitMsgInputs()
	// Make this a manual roll so that the first line is longer than 72 chars,
	// so we can verify that it doesn't get broken.
	manualRollRequester = "manual-requester@google.com"
	result, err := b.Build(from, to, revs, emails, canary, contacts, manualRollRequester)
	require.NoError(t, err)
	require.Equal(t, `Manual roll fake/child/src from aaaaaaaaaaaa to cccccccccccc (2 revisions)

Manual roll requested by manual-requester@google.com

https://fake-child-log/aaaaaaaaaaaa..cccccccccccc

2020-04-17 c@google.com Commit C
2020-04-16 b@google.com Commit B

Also rolling transitive DEPS:
  https://fake-dep1/+log/dddddddddddddddddddddddddddddddddddddddd..eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee
  parent/dep3 from aaaaaaaaaaaa to cccccccccccc

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
https://fake.server.com/r/fake-autoroll
Please CC contact@google.com,reviewer@google.com on the revert to ensure
that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://issues.skia.org/issues/new?component=1389291&template=1850622

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+doc/main/autoroll/README.md

Cq-Include-Trybots: luci.fakeproject.try:some-trybot-on-m92
Cq-Do-Not-Cancel-Tryjobs: true
Bug: fakebugproject:1234,fakebugproject:5678
Tbr: reviewer@google.com
Test: some-test
My-Footer: BlahBlah
My-Other-Footer: Blah
`, result)
}
