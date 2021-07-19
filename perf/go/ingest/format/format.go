// Package format is the format for ingestion files.
package format

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"

	"go.skia.org/infra/go/jsonschema"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/types"

	_ "embed" // For embed functionality.
)

// schema is a json schema for InstanceConfig, it is created by
// running go generate on ./generate/main.go.
//
//go:embed formatSchema.json
var schema []byte

// FileFormatVersion is the version of this ingestion format.
const FileFormatVersion = 1

// ErrFileWrongVersion is returned if the version number in the file is unknown.
var ErrFileWrongVersion = errors.New("File has unknown format version")

// SingleMeasurement is used in Result, see the usage there.
type SingleMeasurement struct {
	// Value is the value part of the key=value pair in a trace id.
	Value string `json:"value"`

	// Measurement is a single measurement from a test run.
	Measurement float32 `json:"measurement"`
}

// Result represents one or more measurements.
//
// Only one of Measurement or Measurements should be populated.
//
// The idea behind Measurements is that you may have more than one metric you
// want to report at the end of running a test, for example you may track the
// fastest time it took to run a test, and also the median and max time. In that
// case you could structure the results as:
//
//    {
//      "key": {
//        "test": "some_test_name"
//      },
//      "measurements": {
//        "ms": [
//          {
//            "value": "min",
//            "measurement": 1.2,
//          },
//          {
//            "value": "max"
//            "measurement": 2.4,
//          },
//          {
//            "value": "median",
//            "measurement": 1.5,
//          }
//        ]
//      }
//    }
//
type Result struct {
	// Key contains key=value pairs will be part of the trace id.
	Key map[string]string `json:"key"`

	// Measurement is a single measurement from a test run.
	Measurement float32 `json:"measurement,omitempty"`

	// Measurements maps from a key to a list of values for that key with
	// associated measurements. Each key=value pair will be part of the trace id.
	Measurements map[string][]SingleMeasurement `json:"measurements,omitempty"`
}

// Format is the struct for decoding ingestion files for all cases that aren't
// nanobench, which uses the BenchData format.
//
// For example, a file that looks like this:
//
//    {
//        "version": 1,
//        "git_hash": "cd5...663",
//        "key": {
//            "config": "8888",
//            "arch": "x86"
//        },
//        "results": [
//            {
//                "key": {
//                    "test": "some_test_name"
//                },
//                "measurements": {
//                    "ms": [
//                        {
//                            "value": "min",
//                            "measurement": 1.2
//                        },
//                        {
//                            "value": "max",
//                            "measurement": 2.4
//                        },
//                        {
//                            "value": "median",
//                            "measurement": 1.5
//                        }
//                    ]
//                }
//            }
//        ]
//    }
//
// Will produce this set of trace ids and values:
//
//    ,arch=x86,config=8888,ms=min,test=some_test_name,      1.2
//    ,arch=x86,config=8888,ms=max,test=some_test_name,      2.4
//    ,arch=x86,config=8888,ms=median,test=some_test_name,   1.5
//
// Key value pair charactes should come from [0-9a-zA-Z\_], particularly note no
// spaces or ':' characters.
type Format struct {
	// Version is the file format version. It should be 1 for this format.
	Version int `json:"version"`

	// GitHash of the repo when these tests were run.
	GitHash string `json:"git_hash"`

	// Issue is the Changelist ID.
	Issue types.CL `json:"issue,omitempty"`

	// Patchset is the tryjob patch identifier. For Gerrit this is an integer
	// serialized as a string.
	Patchset string `json:"patchset,omitempty"`

	// Key contains key=value pairs that are part of all trace ids.
	Key map[string]string `json:"key,omitempty"`

	// Results are all the test results.
	Results []Result `json:"results"`

	// Links are any URLs to further information about this run, e.g. link to a
	// CI run.
	Links map[string]string `json:"links,omitempty"`
}

// Parse parses the stream out of the io.Reader into FileFormat. The caller is
// responsible for calling Close on the reader.
func Parse(r io.Reader) (Format, error) {
	var fileFormat Format
	if err := json.NewDecoder(r).Decode(&fileFormat); err != nil {
		return Format{}, skerr.Wrap(err)
	}
	if fileFormat.Version != FileFormatVersion {
		return Format{}, ErrFileWrongVersion
	}
	return fileFormat, nil
}

// Validate the body of an ingested file against the schema for Format.
//
// If there was an error loading the file a list of schema violations may be
// returned also.
func Validate(ctx context.Context, r io.Reader) ([]string, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to read bytes")
	}
	_, err = Parse(bytes.NewReader(b))
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to parse")
	}

	return jsonschema.Validate(ctx, b, schema)
}
