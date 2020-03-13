// Package format is the format for ingestion files.
package format

const fileFormatVersion = 1

// SingleMeasurement is used in Result, see the usage there.
type SingleMeasurement struct {
	// Value is the value part of the key=value pair in a trace id.
	Value string `json:"value"`

	// Measurement is a single measurement from a test run.
	Measurement float64 `json:"measurement"`
}

// Result represents a set of measurements.
//
// Only one of Measurement or Measurements should be populated.
//
// The idea behind Measurements is that you may have more than one metric you
// want to report at the end of running a test, for example you may track the
// fastest time it took to run a test, and also the median and max time.
// In that case you could structure the results as:
//
//    "result": {
//      "key": {
//	      "test": "some_test_name"
//      }
//	    "measurements": {
//        "ms": {
//	        "min": 1.2,
//          "max": 2.4,
//          "median": 1.5
//        }
//	    }
//    }
//
type Result struct {
	// Key contains key=value pairs will be part of the trace id.
	Key map[string]string `json:"key"`

	// Measurement is a single measurement from a test run.
	Measurement float64 `json:"measurement"`

	// Measurements maps from a key to a list of values for that key with an
	// associated measurement. Each key=value pair will be part of the trace id.
	Measurements map[string][]SingleMeasurement `json:"measurements"`
}

// FileFormat is the struct for decoding ingestion files.
//
// For example, a file that looks like this:
//
// {
//    "version": 1,
//    "git_hash": "cd5...663",
//    "key": {
//        "config": "8888",
//        "arch": "x86",
//    },
//    "results": [
//         {
//          "key": {
//              "test": "some_test_name"
//              },
//            "measurements": {
//                "ms": {
//                    "min": 1.2,
//                      "max": 2.4,
//                      "median": 1.5
//                }
//            }
//        },
//         {
//          "key": {
//              "test": "a_different_test_name"
//              },
//            "measurement": 101.5
//        }
//    ]
// }
//
// Will produce this set of trace ids and values:
//
// ,arch=x86,config=8888,ms=min,test=some_test_name,      1.2
// ,arch=x86,config=8888,ms=max,test=some_test_name,      2.4
// ,arch=x86,config=8888,ms=median,test=some_test_name,   1.5
// ,arch=x86,config=8888,test=a_different_test_name,      101.5
//
type FileFormat struct {
	// Version is the file format version. It should be 1 for this format.
	Version int `json:"version"`

	// GitHash of the repo when these tests were run.
	GitHash string `json:"git_hash"`

	// Key contains key=value pairs will be part of all trace ids.
	Key map[string]string `json:"key"`

	// Results are all the test results.
	Results []Result `json:"results"`

	// Links are any URLs to further information about this run, e.g. link to a
	// CI run.
	Links map[string]string `json:"links"`
}
