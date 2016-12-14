// Parser parses incoming JSON files from Android Testing and converts them
// into a format acceptable to Skia Perf.
package parser

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/android_ingest/go/lookup"
	"go.skia.org/infra/perf/go/ingestcommon"
)

type Incoming struct {
	BuildId     string `json:"build_id"`
	BuildFlavor string `json:"build_flavor"`
	Branch      string `json:"branch"`
	//          map[test name][metric]value
	Metrics map[string]map[string]json.Number `json:"metrics"`
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

	// Convert Incoming into ingestcommon.BenchData, i.e. convert the following:
	//
	//		{
	//			"build_id": "3567162",
	//			"build_flavor": "marlin-userdebug",
	//			"metrics": {
	//				"android.platform.systemui.tests.jank.LauncherJankTests#testAppSwitchGMailtoHome": {
	//				"frame-fps": "9.328892269753897",
	//				"frame-avg-jank": "8.4",
	//				"frame-max-frame-duration": "7.834711093388444",
	//				"frame-max-jank": "10"
	//			},
	//	    ...
	//    }
	//  }
	//
	//  into
	//
	//  {
	//    "gitHash" : "8dcc84f7dc8523dd90501a4feb1f632808337c34",
	//    "key" : {
	//      "build_flavor" : "marlin-userdebug"
	//    },
	//    "results" : {
	//      "android.platform.systemui.tests.jank.LauncherJankTests#testAppSwitchGMailtoHome" : {
	//        "default" : {
	//          "frame-fps": 9.328892269753897,
	//          "frame-avg-jank": 8.4,
	//          "frame-max-frame-duration": 7.834711093388444,
	//          "frame-max-jank": 10
	//          "options" : {
	//            "name" : "android.platform.systemui.tests.jank.LauncherJankTests",
	//            "subtest" : "testAppSwitchGMailtoHome",
	//          },
	//        },
	//      }
	//    }
	//  }
	//
	//
	benchData := &ingestcommon.BenchData{
		Hash: hash,
		Key: map[string]string{
			"build_flavor": in.BuildFlavor,
		},
	}
	for test, metrics := range in.Metrics {
		benchData.Results[test] = ingestcommon.BenchResults{}
		benchData.Results[test]["default"] = ingestcommon.BenchResult{}
		for key, value := range metrics {
			f, err := value.Float64()
			if err != nil {
				glog.Errorf("Couldn't parse %q as a float64: %s", value.String(), err)
				continue
			}
			benchData.Results[test]["default"][key] = f
		}
		parts := strings.Split(test, "#")
		if len(parts) != 2 {
			glog.Errorf("Test name didn't have a single # separator: %q", test)
			continue
		}
		benchData.Results[test]["default"]["options"] = map[string]string{
			"name":    parts[0],
			"subtest": parts[1],
		}
	}

	b, err := json.Marshal(benchData)
	if err != nil {
		return "", fmt.Errorf("Failed to find JSON encode results: %s", err)
	}
	return string(b), nil
}
