package backends

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/pinpoint/go/common"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
)

func mockIssueClient(t *testing.T) *issueTrackerTransport {
	tmpl, err := configureTemplates()
	require.NoError(t, err)
	require.NotNil(t, tmpl)

	return &issueTrackerTransport{
		client: nil,
		tmpl:   tmpl,
	}
}

func TestFillTemplate_NoCulprit_EmptyString(t *testing.T) {
	c := mockIssueClient(t)

	var input []*pinpoint_proto.CombinedCommit
	resp, err := c.fillTemplate(input)
	assert.NoError(t, err)
	assert.Equal(t, "", resp)

	input = []*pinpoint_proto.CombinedCommit{}
	resp, err = c.fillTemplate(input)
	assert.NoError(t, err)
	assert.Equal(t, "", resp)
}

func TestFillTemplate_MultiCulprit_TemplateFilled(t *testing.T) {
	c := mockIssueClient(t)

	input := []*pinpoint_proto.CombinedCommit{
		{
			Main: common.NewChromiumCommit("1a9897ca56513579444c0411ffe910b4d26b894c"),
			ModifiedDeps: []*pinpoint_proto.Commit{
				{
					Repository: "https://chromium.googlesource.com/v8/v8.git",
					GitHash:    "2c06a2a008c123b4f33ffcad2cf4a3c9bcc2970f",
				},
			},
		},
		{
			Main: common.NewChromiumCommit("8c2a7b436376c9283f85d543a40f710b2aa18e34"),
		},
	}

	const expectedResp = `[BETA] Skia-Pinpoint Results:
*Please ignore the results printed from this comment as this is in BETA mode*

Found significant difference(s) at 2 commit(s).

Understanding performance regressions: http://g.co/ChromePerformanceRegressions

Culprits:
* https://chromium.googlesource.com/chromium/src.git/+/1a9897ca56513579444c0411ffe910b4d26b894c https://chromium.googlesource.com/v8/v8.git/+/2c06a2a008c123b4f33ffcad2cf4a3c9bcc2970f
* https://chromium.googlesource.com/chromium/src.git/+/8c2a7b436376c9283f85d543a40f710b2aa18e34

If you think Pinpoint blamed the wrong commit, please add issue to
"Chromeperf-CulpritDetection-NeedsAttention" hotlist and unassign
yourself so that a sheriff can help diagnose.`

	resp, err := c.fillTemplate(input)
	assert.NoError(t, err)
	assert.Equal(t, resp, expectedResp)
}
