// Package gcsfileloader implements fileloader.FileLoader for Google Cloud Storage.
package gcsfileloader

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs/test_gcsclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
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
		  "min_ms": 2.223606
		},
		"565": {
		  "min_ms": 2.215988
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
	return parser.New(instanceConfig)
}
func TestGetSamples_Success(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	const sourceFilePath = "/path/file.json"
	const sourceFileName = "gs://skia-perf" + sourceFilePath

	gcsclient := &test_gcsclient.GCSClient{}
	r := bytes.NewBufferString(sourceFileBody)
	gcsclient.On("FileReader", testutils.AnyContext, sourceFilePath).Return(ioutil.NopCloser(r), nil)

	g := New(gcsclient, ingestParser())
	sampleSet, err := g.GetSamples(ctx, sourceFileName)
	require.NoError(t, err)
	require.Equal(t, parser.SamplesSet{}, sampleSet)
}
