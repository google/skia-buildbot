// Package gcssamplesloader implements samplesloader.SamplesLoader for Google Cloud Storage.
package gcssamplesloader

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs/test_gcsclient"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/ingest/parser"
)

// sourceFileBody is a source file that contains the following keys:
//
//    ,config=8888,name=mytest,sub_result=min_ms,
//    ,config=565,name=mytest,sub_result=min_ms,
const sourceFileBody = `{
    "gitHash": "fe4a4029a080bc955e9588d05a6cd9eb490845d4",
    "issue": "327697",
    "patchset": "1",
    "results": {
      "mytest": {
        "8888": {
          "min_ms": 2.223606,
          "samples": [
            0.2057559490203857,
            0.1993551254272461,
            0.198721170425415
            ]
        },
        "565": {
          "min_ms": 2.215988,
          "samples": [
            0.199286937713623,
            0.1990699768066406,
            0.1992120742797852
          ]
        }
      }
    }
  }
`

// Returns ingestion parser.
func ingestParser() *parser.Parser {
	instanceConfig := &config.InstanceConfig{
		IngestionConfig: config.IngestionConfig{
			Branches: nil,
		},
	}
	return parser.New(instanceConfig.IngestionConfig.Branches)
}

func TestLoad_Success(t *testing.T) {
	ctx := context.Background()

	const sourceFilePath = "path/file.json"
	const sourceFileName = "gs://skia-perf/" + sourceFilePath

	gcsclient := &test_gcsclient.GCSClient{}
	r := bytes.NewBufferString(sourceFileBody)
	gcsclient.On("FileReader", testutils.AnyContext, sourceFilePath).Return(ioutil.NopCloser(r), nil)

	g := New(gcsclient, ingestParser())
	sampleSet, err := g.Load(ctx, sourceFileName)
	require.NoError(t, err)
	expected := parser.SamplesSet{
		",config=565,sub_result=min_ms,test=mytest,": parser.Samples{
			Params: paramtools.Params{"config": "565", "sub_result": "min_ms", "test": "mytest"},
			Values: []float64{0.199286937713623, 0.1990699768066406, 0.1992120742797852},
		},
		",config=8888,sub_result=min_ms,test=mytest,": parser.Samples{
			Params: paramtools.Params{"config": "8888", "sub_result": "min_ms", "test": "mytest"},
			Values: []float64{0.2057559490203857, 0.1993551254272461, 0.198721170425415},
		},
	}
	require.Equal(t, expected, sampleSet)
}

func TestLoad_FileReaderFails_Failure(t *testing.T) {
	ctx := context.Background()
	const sourceFilePath = "path/file.json"
	const sourceFileName = "gs://skia-perf/" + sourceFilePath
	gcsclient := &test_gcsclient.GCSClient{}
	gcsclient.On("FileReader", testutils.AnyContext, sourceFilePath).Return(nil, fmt.Errorf("something broke"))

	g := New(gcsclient, ingestParser())
	_, err := g.Load(ctx, sourceFileName)
	require.Contains(t, err.Error(), "Failed to load from storage")
}

func TestLoad_InvalidFilenameURL_Failure(t *testing.T) {
	ctx := context.Background()
	gcsclient := &test_gcsclient.GCSClient{}
	g := New(gcsclient, ingestParser())
	_, err := g.Load(ctx, "::: not a valid url :::")
	require.Contains(t, err.Error(), "Failed to parse filename")
}

func TestLoad_InvalidJSON_Failure(t *testing.T) {
	ctx := context.Background()

	const sourceFilePath = "path/file.json"
	const sourceFileName = "gs://skia-perf/" + sourceFilePath

	gcsclient := &test_gcsclient.GCSClient{}
	r := bytes.NewBufferString("}this isn't valid JSON{")
	gcsclient.On("FileReader", testutils.AnyContext, sourceFilePath).Return(ioutil.NopCloser(r), nil)

	g := New(gcsclient, ingestParser())
	_, err := g.Load(ctx, sourceFileName)
	require.Contains(t, err.Error(), "Failed to parse samples from file")
}
