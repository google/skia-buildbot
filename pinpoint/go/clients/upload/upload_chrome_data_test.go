package upload

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/pinpoint/go/backends/mocks"
)

const (
	testProject   = "chromeperf"
	testDataset   = "experiment"
	testTableName = "benchmark_results"
)

type SampleTableDefinition struct {
	ParentWorkflowID string    `bigquery:"workflow_id"`
	Benchmark        string    `bigquery:"benchmark"`
	Chart            string    `bigquery:"chart"`
	SwarmingTaskID   string    `bigquery:"swarming_task_id"`
	SampleValues     []float64 `bigquery:"sample_values"`
}

func TestCreateTable_ValidReq_NoError(t *testing.T) {
	ctx := context.Background()

	mockClient := &mocks.BigQueryClient{}
	mockClient.On("CreateTable", testutils.AnyContext, testDataset, testTableName, SampleTableDefinition{}).Return(nil)

	client := &uploadChromeDataClient{
		Project:   testProject,
		DatasetID: testDataset,
		TableName: testTableName,
		client:    mockClient,
	}

	err := client.CreateTableFromStruct(ctx, &CreateTableRequest{
		Definition: SampleTableDefinition{},
	})
	require.NoError(t, err)
}

func TestInsert_ValidReq_NoError(t *testing.T) {
	ctx := context.Background()

	rows := []interface{}{
		&SampleTableDefinition{
			ParentWorkflowID: "workflow1",
			Benchmark:        "foo",
			Chart:            "bar",
			SwarmingTaskID:   "swarming1",
			SampleValues:     []float64{10.001, 11.001, 12.002},
		},
		&SampleTableDefinition{
			ParentWorkflowID: "workflow2",
			Benchmark:        "foo",
			Chart:            "baz",
			SwarmingTaskID:   "swarming2",
			SampleValues:     []float64{9.001, 8.002, 7.003},
		},
	}

	mockClient := &mocks.BigQueryClient{}
	mockClient.On("Insert", testutils.AnyContext, testDataset, testTableName, rows).Return(nil)

	client := &uploadChromeDataClient{
		Project:   testProject,
		DatasetID: testDataset,
		TableName: testTableName,
		client:    mockClient,
	}

	err := client.Insert(ctx, &InsertRequest{
		Items: rows,
	})
	require.NoError(t, err)
}
