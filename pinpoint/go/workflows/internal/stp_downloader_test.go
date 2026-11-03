package internal

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"golang.org/x/net/html"

	"go.skia.org/infra/pinpoint/go/backends/mocks"
)

// stpResourcesPage keeps the parts of https://developer.apple.com/safari/resources/
// that extractFromHtml reads, including the &nbsp; in the link labels.
const stpResourcesPage = `<html><body>
<div class="row gutter text-left">
  <div class="column">
    <p class="sosumi no-margin-bottom margin-top-small">Release</p>
    <p class="smaller lighter no-margin">255</p>
  </div>
  <div class="column">
    <p class="sosumi no-margin-bottom margin-top-small">Posted</p>
    <p class="smaller lighter no-margin">October 22, 2026</p>
  </div>
</div>
<ul>
  <li class="dmg"><a class="inline" href="https://secure-appldnld.apple.com/STP/a/SafariTechPreview255.dmg">Safari Technology Preview<br />for macOS&nbsp;27</a><br /><span>Requires macOS&nbsp;27 or later</span></li>
  <li class="dmg"><a class="inline" href="https://secure-appldnld.apple.com/STP/b/SafariTechnologyPreview.dmg">Safari Technology Preview<br />for macOS&nbsp;26</a><br /><span>Requires macOS&nbsp;26 or later</span></li>
</ul>
</body></html>`

func macBot(osValues ...string) *apipb.BotInfo {
	return &apipb.BotInfo{Dimensions: []*apipb.StringListPair{
		{Key: "pool", Value: []string{"chrome.tests.pinpoint-cbb"}},
		{Key: "os", Value: osValues},
	}}
}

func TestExtractFromHtml_LinkLabelsUseNbsp_MapsEachMacOSVersionToItsLink(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(stpResourcesPage))
	require.NoError(t, err)

	ri := extractFromHtml(doc)

	assert.Equal(t, "255", ri.release)
	assert.Equal(t, map[string]string{
		"27": "https://secure-appldnld.apple.com/STP/a/SafariTechPreview255.dmg",
		"26": "https://secure-appldnld.apple.com/STP/b/SafariTechnologyPreview.dmg",
	}, ri.links)
}

func TestFleetMacOSMajor_AllBotsOnSameMajorVersion_ReturnsIt(t *testing.T) {
	sc := mocks.NewSwarmingClient(t)
	sc.On("FetchFreeBots", mock.Anything, mock.Anything).Return([]*apipb.BotInfo{
		macBot("Mac", "Mac-27", "Mac-27.0", "Mac-27.0.1", "Mac-27.0.1-26A434"),
		macBot("Mac", "Mac-27", "Mac-27.1", "Mac-27.1-26B12"),
	}, nil)

	major, err := fleetMacOSMajor(context.Background(), sc)

	require.NoError(t, err)
	assert.Equal(t, "27", major)
}

func TestFleetMacOSMajor_FleetMidUpgrade_ReturnsError(t *testing.T) {
	sc := mocks.NewSwarmingClient(t)
	sc.On("FetchFreeBots", mock.Anything, "mac-m3-pro-perf-cbb").Return([]*apipb.BotInfo{
		macBot("Mac", "Mac-26", "Mac-26.5", "Mac-26.5.2"),
	}, nil)
	sc.On("FetchFreeBots", mock.Anything, mock.Anything).Return([]*apipb.BotInfo{
		macBot("Mac", "Mac-27", "Mac-27.0", "Mac-27.0.1"),
	}, nil)

	_, err := fleetMacOSMajor(context.Background(), sc)

	require.ErrorContains(t, err, "got [26 27]")
}

func TestFleetMacOSMajor_NoLiveBots_ReturnsError(t *testing.T) {
	sc := mocks.NewSwarmingClient(t)
	sc.On("FetchFreeBots", mock.Anything, mock.Anything).Return([]*apipb.BotInfo{}, nil)

	_, err := fleetMacOSMajor(context.Background(), sc)

	require.ErrorContains(t, err, "got []")
}
