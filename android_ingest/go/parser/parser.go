// Package parser parses incoming JSON files from Android Testing and converts them
// into a format acceptable to Skia Perf.
package parser

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/ingest/format"
)

var (
	// ErrIgnorable is returned from Convert if the file can safely be ignored.
	ErrIgnorable = errors.New("File should be ignored.")
)

// Incoming is the JSON structure of the data sent to us from the Android
// testing infrastructure.
type Incoming struct {
	BuildId        string `json:"build_id"`
	BuildFlavor    string `json:"build_flavor"`
	Branch         string `json:"branch"`
	DeviceName     string `json:"device_name"`
	SDKReleaseName string `json:"sdk_release_name"`
	JIT            string `json:"jit"`

	// Metrics is a map[test name]map[metric]value, where value
	// is a string encoded float, thus the use of json.Number.
	Metrics map[string]map[string]json.Number `json:"metrics"`
}

// Parse the 'incoming' stream into an *Incoming struct.
func Parse(incoming io.Reader) (*Incoming, error) {
	ret := &Incoming{}
	if err := json.NewDecoder(incoming).Decode(ret); err != nil {
		return nil, fmt.Errorf("Failed to decode incoming JSON: %s", err)
	}
	return ret, nil
}

// Lookup is an interface for looking up a git hashes from a buildid.
//
// The *lookup.Cache satisfies this interface.
type Lookup interface {
	Lookup(buildid int64) (string, error)
}

// Converter converts a serialized *Incoming into
// an *format.BenchData.
type Converter struct {
	lookup Lookup
}

// New creates a new *Converter.
//
func New(lookup Lookup) *Converter {
	return &Converter{
		lookup: lookup,
	}
}

// Convert the serialize *Incoming JSON into an *format.BenchData.
func (c *Converter) Convert(incoming io.Reader, txLogName string) (*format.BenchData, error) {
	b, err := ioutil.ReadAll(incoming)
	if err != nil {
		return nil, fmt.Errorf("Failed to read during convert %q: %s", txLogName, err)
	}
	reader := bytes.NewReader(b)
	in, err := Parse(reader)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse during convert %q: %s", txLogName, err)
	}
	metrics2.GetCounter("androidingest_upload_received", map[string]string{"branch": in.Branch}).Inc(1)
	sklog.Infof("POST for filename %q buildid: %s branch: %s flavor: %s num metrics: %d", txLogName, in.BuildId, in.Branch, in.BuildFlavor, len(in.Metrics))

	// Files where the BuildId is prefixed with "P" are presubmits and don't
	// produce any data and can be ignored.
	if strings.HasPrefix(in.BuildId, "P") {
		return nil, ErrIgnorable
	}
	buildid, err := strconv.ParseInt(in.BuildId, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse buildid %q: %q %q %s", in.BuildId, txLogName, in.Branch, err)
	}
	hash, err := c.lookup.Lookup(buildid)
	if err != nil {
		return nil, fmt.Errorf("Failed to find matching hash for buildid %d: %q %q %s", buildid, txLogName, in.Branch, err)
	}

	// Convert Incoming into format.BenchData, i.e. convert the following:
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
	// Note that the incoming data doesn't have a concept similar to "config" so we just
	// use a value of "default" for config for now.
	benchData := &format.BenchData{
		Hash: hash,
		Key: map[string]string{
			"build_flavor": in.BuildFlavor,
		},
		Results: map[string]format.BenchResults{},
	}
	if in.DeviceName != "" {
		benchData.Key["device_name"] = in.DeviceName
	}
	if in.SDKReleaseName != "" {
		benchData.Key["sdk_release_name"] = in.SDKReleaseName
	}
	if in.JIT != "" {
		benchData.Key["jit"] = in.JIT
	}

	// Record the branch name.
	benchData.Key["branch"] = in.Branch

	for test, metrics := range in.Metrics {
		benchData.Results[test] = format.BenchResults{}
		benchData.Results[test]["default"] = format.BenchResult{}
		for key, value := range metrics {
			f, err := value.Float64()
			if err != nil {
				sklog.Warningf("Couldn't parse %q as a float64: %s", value.String(), err)
				continue
			}
			benchData.Results[test]["default"][key] = f
		}
	}
	if len(benchData.Results) == 0 {
		return nil, fmt.Errorf("Failed to extract any data from incoming file: %q", txLogName)
	}
	sklog.Infof("Found %d metrics of %d incoming metrics in branch %q buildid %q in file %q", len(benchData.Results), len(in.Metrics), in.Branch, in.BuildId, txLogName)
	metrics2.GetCounter("androidingest_upload_success", map[string]string{"branch": in.Branch}).Inc(1)

	return benchData, nil
}
