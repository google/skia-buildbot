package parser

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	r := bytes.NewBufferString(INCOMING)
	in, err := Parse(r)
	assert.NoError(t, err)
	assert.Equal(t, "google-marlin-marlin-O", in.Branch)
	assert.Len(t, in.Metrics, 7)
	assert.Equal(t, "8.4", in.Metrics["android.platform.systemui.tests.jank.LauncherJankTests#testAppSwitchGMailtoHome"]["frame-avg-jank"])
}

const INCOMING = `{
	"build_id": "3567162",
	"build_flavor": "marlin-userdebug",
	"metrics": {
		"android.platform.systemui.tests.jank.LauncherJankTests#testAppSwitchGMailtoHome": {
			"frame-fps": "9.328892269753897",
			"frame-avg-jank": "8.4",
			"frame-max-frame-duration": "7.834711093388444",
			"frame-max-jank": "10"
		},
		"android.platform.systemui.tests.jank.LauncherJankTests#testHomeScreenSwipe": {
			"gfx-avg-slow-ui-thread": "0.10191099340499558",
			"gfx-max-slow-bitmap-uploads": "0.0",
			"gfx-max-frame-time-95": "8",
			"gfx-max-frame-time-50": "5",
			"gfx-max-slow-ui-thread": "0.25510204081632654",
			"gfx-avg-frame-time-50": "5.0",
			"gfx-max-jank": "0.26",
			"gfx-avg-slow-draw": "0.0",
			"gfx-avg-frame-time-95": "7.4",
			"gfx-max-frame-time-90": "7",
			"gfx-avg-frame-time-90": "6.8",
			"gfx-avg-jank": "0.10200000000000001",
			"gfx-max-missed-vsync": "0.0",
			"gfx-avg-slow-bitmap-uploads": "0.0",
			"gfx-max-high-input-latency": "0.0",
			"gfx-max-frame-time-99": "12",
			"gfx-avg-missed-vsync": "0.0",
			"gfx-avg-frame-time-99": "10.4",
			"gfx-max-slow-draw": "0.0",
			"gfx-avg-high-input-latency": "0.0"
		},
		"android.platform.systemui.tests.jank.LauncherJankTests#testWidgetsContainerFling": {
			"gfx-avg-slow-ui-thread": "0.0968528680643497",
			"gfx-max-slow-bitmap-uploads": "0.0",
			"gfx-max-frame-time-95": "9",
			"gfx-max-frame-time-50": "5",
			"gfx-max-slow-ui-thread": "0.24271844660194172",
			"gfx-avg-frame-time-50": "5.0",
			"gfx-max-jank": "0.5",
			"gfx-avg-slow-draw": "0.0",
			"gfx-avg-frame-time-95": "8.2",
			"gfx-max-frame-time-90": "8",
			"gfx-avg-frame-time-90": "7.2",
			"gfx-avg-jank": "0.294",
			"gfx-max-missed-vsync": "0.24271844660194172",
			"gfx-avg-slow-bitmap-uploads": "0.0",
			"gfx-max-high-input-latency": "0.0",
			"gfx-max-frame-time-99": "15",
			"gfx-avg-missed-vsync": "0.14539655738473806",
			"gfx-avg-frame-time-99": "11.0",
			"gfx-max-slow-draw": "0.0",
			"gfx-avg-high-input-latency": "0.0"
		},
		"android.platform.systemui.tests.jank.LauncherJankTests#testAllAppsContainerSwipe": {
			"gfx-avg-slow-ui-thread": "0.07554138508437006",
			"gfx-max-slow-bitmap-uploads": "0.07598784194528875",
			"gfx-max-frame-time-95": "9",
			"gfx-max-frame-time-50": "5",
			"gfx-max-slow-ui-thread": "0.1508295625942685",
			"gfx-avg-frame-time-50": "5.0",
			"gfx-max-jank": "0.3",
			"gfx-avg-slow-draw": "0.045592705167173245",
			"gfx-avg-frame-time-95": "8.2",
			"gfx-max-frame-time-90": "8",
			"gfx-avg-frame-time-90": "7.4",
			"gfx-avg-jank": "0.16599999999999998",
			"gfx-max-missed-vsync": "0.15232292460015232",
			"gfx-avg-slow-bitmap-uploads": "0.01519756838905775",
			"gfx-max-high-input-latency": "0.0",
			"gfx-max-frame-time-99": "11",
			"gfx-avg-missed-vsync": "0.07567937219606331",
			"gfx-avg-frame-time-99": "10.6",
			"gfx-max-slow-draw": "0.22796352583586624",
			"gfx-avg-high-input-latency": "0.0"
		},
		"android.platform.systemui.tests.jank.LauncherJankTests#testAppSwitchChrometoHome": {
			"frame-fps": "9.059377622237943",
			"frame-avg-jank": "8.6",
			"frame-max-frame-duration": "11.048077785923113",
			"frame-max-jank": "9"
		},
		"android.platform.systemui.tests.jank.LauncherJankTests#testAppSwitchPhotostoHome": {
			"frame-fps": "9.342556065428203",
			"frame-avg-jank": "8.0",
			"frame-max-frame-duration": "7.633792937351717",
			"frame-max-jank": "9"
		},
		"android.platform.systemui.tests.jank.LauncherJankTests#testOpenAllAppsContainer": {
			"gfx-avg-slow-ui-thread": "5.040813095770279",
			"gfx-max-slow-bitmap-uploads": "0.0",
			"gfx-max-frame-time-95": "24",
			"gfx-max-frame-time-50": "7",
			"gfx-max-slow-ui-thread": "5.352112676056338",
			"gfx-avg-frame-time-50": "7.0",
			"gfx-max-jank": "8.17",
			"gfx-avg-slow-draw": "1.5528189571212099",
			"gfx-avg-frame-time-95": "22.4",
			"gfx-max-frame-time-90": "14",
			"gfx-avg-frame-time-90": "12.8",
			"gfx-avg-jank": "7.148000000000001",
			"gfx-max-missed-vsync": "3.867403314917127",
			"gfx-avg-slow-bitmap-uploads": "0.0",
			"gfx-max-high-input-latency": "0.0",
			"gfx-max-frame-time-99": "61",
			"gfx-avg-missed-vsync": "3.4349243386335053",
			"gfx-avg-frame-time-99": "51.4",
			"gfx-max-slow-draw": "2.2535211267605635",
			"gfx-avg-high-input-latency": "0.0"
		}
	},
	"branch": "google-marlin-marlin-O"
}`
