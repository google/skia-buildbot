package commit_msg

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestNamedTemplateCanary(t *testing.T) {
	unittest.SmallTest(t)

	b := fakeBuilder(t)
	b.cfg.Template = &config.CommitMsgConfig_BuiltIn_{
		BuiltIn: config.CommitMsgConfig_CANARY,
	}
	result, err := b.Build(FakeCommitMsgInputs())
	require.NoError(t, err)
	require.Equal(t, `Canary roll fake/child/src to cccccccccccc

https://fake-child-log/aaaaaaaaaaaa..cccccccccccc

DO_NOT_SUBMIT: This canary roll is only for testing

Documentation for Autoroller Canaries is here:
go/autoroller-canary-bots (Googlers only)

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Commit: false
Cq-Include-Trybots: some-trybot-on-m92
Cq-Do-Not-Cancel-Tryjobs: true
`, result)
}
