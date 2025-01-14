// Package format is the format for ingestion files.
package format

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"maps"

	"go.skia.org/infra/go/jsonschema"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
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

	// Links are any URLs to further information about this measurement.
	// The key is the display name for the link and the value is the URL.
	// Eg: Links["Search Engine"] = "https://www.google.com"
	Links map[string]string `json:"links,omitempty"`
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
//	{
//	  "key": {
//	    "test": "some_test_name"
//	  },
//	  "measurements": {
//	    "ms": [
//	      {
//	        "value": "min",
//	        "measurement": 1.2,
//	      },
//	      {
//	        "value": "max"
//	        "measurement": 2.4,
//	      },
//	      {
//	        "value": "median",
//	        "measurement": 1.5,
//	      }
//	    ]
//	  }
//	}
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
//	{
//	    "version": 1,
//	    "git_hash": "cd5...663",
//	    "key": {
//	        "config": "8888",
//	        "arch": "x86"
//	    },
//	    "results": [
//	        {
//	            "key": {
//	                "test": "a_test_with_just_a_single_measurement",
//	                "units": "s"
//	            },
//	            "measurement": 123.4
//	        },
//	        {
//	            "key": {
//	                "test": "draw_a_circle",
//	                "units": "ms"
//	            },
//	            "measurements": {
//	                "stat": [
//	                    {
//	                        "value": "min",
//	                        "measurement": 1.2
//	                    },
//	                    {
//	                        "value": "max",
//	                        "measurement": 2.4
//	                    },
//	                    {
//	                        "value": "median",
//	                        "measurement": 1.5
//	                    }
//	                ]
//	            }
//	        },
//	        {
//	            "key": {
//	                "test": "draw_my_animation",
//	                "units": "Hz"
//	            },
//	            "measurements": {
//	                "stat": [
//	                    {
//	                        "value": "min",
//	                        "measurement": 20
//	                    },
//	                    {
//	                        "value": "max",
//	                        "measurement": 30
//	                    },
//	                    {
//	                        "value": "median",
//	                        "measurement": 22
//	                    }
//	                ]
//	            }
//	        }
//	    ],
//	    "links": {
//	        "details": "https://example.com/a-link-to-details-about-this-test-run"
//	    }
//	}
//
// Will produce this set of trace ids and values:
//
//	Hash:
//	  cd5...663
//	Measurements:
//	  ,arch=x86,config=8888,test=a_test_with_just_a_single_measurement,units=s, = 123.4
//	  ,arch=x86,config=8888,stat=min,test=draw_a_circle,units=ms, = 1.2
//	  ,arch=x86,config=8888,stat=max,test=draw_a_circle,units=ms, = 2.4
//	  ,arch=x86,config=8888,stat=median,test=draw_a_circle,units=ms, = 1.5
//	  ,arch=x86,config=8888,stat=min,test=draw_my_animation,units=Hz, = 20
//	  ,arch=x86,config=8888,stat=max,test=draw_my_animation,units=Hz, = 30
//	  ,arch=x86,config=8888,stat=median,test=draw_my_animation,units=Hz, = 22
//	Links:
//	  details: https://example.com/a-link-to-details-about-this-test-run
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

// GetLinksForMeasurement returns a list of links from the data. This includes
// the links common for all measurements plus any links specified in the measurement
// for the given trace id.
func (f Format) GetLinksForMeasurement(traceID string) map[string]string {
	links := map[string]string{}
	if f.Links != nil {
		links = maps.Clone(f.Links)
	}
	traceParamSet := paramtools.NewParamSet(paramtools.NewParams(traceID))
	keyParams := paramtools.Params(f.Key)
	for _, result := range f.Results {
		p := keyParams.Copy()
		p.Add(result.Key)
		for key, measurements := range result.Measurements {
			for _, measurement := range measurements {
				singleParam := p.Copy()
				singleParam[key] = measurement.Value
				singleParam = query.ForceValid(singleParam)
				measurementParamSet := paramtools.NewParamSet(singleParam)
				// At this point we have gathered all the params for the given measurement.
				// Now we can simply check if these params match the ones in the traceID to
				// verify if we have the right measurement. Also adding a size check to verify
				// equivalence since the Matches call also works if there is a subset match.
				if traceParamSet.Size() == measurementParamSet.Size() &&
					traceParamSet.Matches(measurementParamSet) {
					if len(measurement.Links) > 0 {
						for id, url := range measurement.Links {
							links[id] = url
						}
					}

					// Since we already matched with the correct measurement, no further
					// traversal is necessary.
					break
				}
			}
		}

	}
	return links
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

// Write encodes the Format object into json and writes it
// into the supplied writer.
func (f Format) Write(w io.Writer) error {
	if err := json.NewEncoder(w).Encode(f); err != nil {
		return skerr.Wrap(err)
	}

	return nil
}

// Validate the body of an ingested file against the schema for Format.
//
// If there was an error loading the file a list of schema violations may be
// returned also.
func Validate(r io.Reader) ([]string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to read bytes")
	}
	_, err = Parse(bytes.NewReader(b))
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to parse")
	}

	return jsonschema.Validate(b, schema)
}
