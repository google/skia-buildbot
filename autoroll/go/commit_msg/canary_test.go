package commit_msg

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
)

func TestNamedTemplateCanary(t *testing.T) {

	b := fakeBuilder(t)
	b.cfg.BuiltIn = config.CommitMsgConfig_CANARY
	result, err := b.Build(FakeCommitMsgInputs())
	require.NoError(t, err)
	require.Equal(t, `Canary roll fake/child/src to cccccccccccc

https://fake-child-log/aaaaaaaaaaaa..cccccccccccc

DO_NOT_SUBMIT: This canary roll is only for testing

Documentation for Autoroller Canaries is here:
go/autoroller-canary-bots (Googlers only)

To report a problem with the AutoRoller itself, please file a bug:
https://issues.skia.org/issues/new?component=1389291&template=1850622

Commit: false
Cq-Include-Trybots: luci.fakeproject.try:some-trybot
Cq-Do-Not-Cancel-Tryjobs: true
`, result)
}

func TestNamedTemplateCanary_WithExternalChangeId(t *testing.T) {

	b := fakeBuilder(t)
	b.cfg.BuiltIn = config.CommitMsgConfig_CANARY
	from, to, revs, reviewers, contacts, canary, manualRollRequester := FakeCommitMsgInputs()
	to.ExternalChangeId = "12345"

	result, err := b.Build(from, to, revs, reviewers, contacts, canary, manualRollRequester)
	require.NoError(t, err)
	require.Equal(t, `Canary roll fake/child/src to cccccccccccc

https://fake-child-log/aaaaaaaaaaaa..cccccccccccc

This canary roll also includes patch from change 12345

DO_NOT_SUBMIT: This canary roll is only for testing

Documentation for Autoroller Canaries is here:
go/autoroller-canary-bots (Googlers only)

To report a problem with the AutoRoller itself, please file a bug:
https://issues.skia.org/issues/new?component=1389291&template=1850622

Commit: false
Cq-Include-Trybots: luci.fakeproject.try:some-trybot
Cq-Do-Not-Cancel-Tryjobs: true
`, result)
}
