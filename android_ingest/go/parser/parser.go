// Parser parses incoming JSON files from Android Testing and converts them
// into a format acceptable to Skia Perf.
package parser

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"go.skia.org/infra/android_ingest/go/lookup"
	"go.skia.org/infra/perf/go/ingestcommon"
)

type Incoming struct {
	BuildId     string                            `json:"build_id"`
	BuildFlavor string                            `json:"build_flavor"`
	Branch      string                            `json:"branch"`
	Metrics     map[string]map[string]json.Number `json:"metrics"`
}

func Parse(incoming io.Reader) (*Incoming, error) {
	ret := &Incoming{}
	if err := json.NewDecoder(incoming).Decode(ret); err != nil {
		return nil, fmt.Errorf("Failed to decode incoming JSON: %s", err)
	}
	return ret, nil
}

type Converter struct {
	lookup *lookup.Cache
	branch string
}

func New(lookup *lookup.Cache, branch string) *Converter {
	return &Converter{
		lookup: lookup,
		branch: branch,
	}
}

func (c *Converter) Convert(incoming io.Reader) (string, error) {
	in, err := Parse(incoming)
	if err != nil {
		return "", fmt.Errorf("Failed to parse during convert: %s", err)
	}
	buildid, err := strconv.ParseInt(in.BuildId, 10, 64)
	if err != nil {
		return "", fmt.Errorf("Failed to parse buildid %q: %s", in.BuildId, err)
	}
	hash, err := c.lookup.Lookup(buildid)
	if err != nil {
		return "", fmt.Errorf("Failed to find matching hash for buildid %d: %s", buildid, err)
	}
	if in.Branch != c.branch {
		return "", fmt.Errorf("Found data for a branch we weren't expecting %q: %s", in.Branch, err)
	}

	/*

				test		"android.platform.systemui.tests.jank.LauncherJankTests#testAppSwitchGMailtoHome": {
				name    "android.platform.systemui.tests.jank.LauncherJankTests"
		    config  "default"

		      {
		         "gitHash" : "8dcc84f7dc8523dd90501a4feb1f632808337c34",
		         "key" : {
		            "build_flavor" : "marlin-userdebug"
		         },
		         "results" : {
				test:  "android.platform.systemui.tests.jank.LauncherJankTests_testAppSwitchGMailtoHome" : {
				config:  "default" : {
		                "frame-fps": 9.328892269753897,
		                "frame-avg-jank": 8.4,
		                "frame-max-frame-duration": 7.834711093388444,
		                "frame-max-jank": 10
				            "options" : {
				name:          "name" : "android.platform.systemui.tests.jank.LauncherJankTests",
				subtest:       "subtest" : "testAppSwitchGMailtoHome",
				            },
				         },


		      {
		         "build_number" : "2",
		         "gitHash" : "8dcc84f7dc8523dd90501a4feb1f632808337c34",
		         "key" : {
		            "arch" : "x86_64",
		            "compiler" : "Clang",
		            "cpu_or_gpu" : "CPU",
		            "cpu_or_gpu_value" : "AVX2",
		            "model" : "GCE",
		            "os" : "Ubuntu"
		         },
		         "no_buildbot" : "True",
		         "results" : {
				test:  "AJ_Digital_Camera.svg_1.1_1000_1000" : {
				config:  "565" : {
				            "min_ms" : 4.043701171875,
				            "options" : {
				               "bench_type" : "playback",
				name:          "name" : "AJ_Digital_Camera.svg",
				               "source_type" : "svg"
				            },
				         },
				         "8888" : {

	*/

	// Convert Incoming into ingestcommon.BenchData.
	benchData := &ingestcommon.BenchData{
		Hash: hash,
		Key: map[string]string{
			"build_flavor": in.BuildFlavor,
		},
	}
	// First build Key.

	b, err := json.Marshal(benchData)
	if err != nil {
		return "", fmt.Errorf("Failed to find JSON encode results: %s", err)
	}
	return string(b), nil
}
