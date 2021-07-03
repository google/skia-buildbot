// Command-line application for interacting with Perf.
package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/perf-tool/application/mocks"
)

func createInstanceConfigFile(t *testing.T) string {
	instanceConfig := &config.InstanceConfig{
		URL: "http://",
		IngestionConfig: config.IngestionConfig{
			Branches: []string{},
			SourceConfig: config.SourceConfig{
				Sources: []string{},
			},
		},
	}
	f, err := ioutil.TempFile("", "perf-tool")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.Remove(f.Name())) })
	err = json.NewEncoder(f).Encode(instanceConfig)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

func TestActualMain_ConfigCreatePubSubTopics_Success(t *testing.T) {
	unittest.SmallTest(t)
	app := &mocks.Application{}
	app.On("ConfigCreatePubSubTopics", mock.AnythingOfType("*config.InstanceConfig")).Return(nil)

	filename := createInstanceConfigFile(t)

	os.Args = []string{"perf-tool", "config", "create-pubsub-topics", "--config_filename=" + filename}
	actualMain(app)
	app.AssertExpectations(t)
}
