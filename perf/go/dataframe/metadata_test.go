package dataframe

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/perf/go/config"
	traceStoreMocks "go.skia.org/infra/perf/go/tracestore/mocks"
	"go.skia.org/infra/perf/go/types"
)

func TestGetMetadataForTraces_Success(t *testing.T) {
	// The dataframe contains 3 commits 1,2,3 and 2 traces
	//
	//  [",arch=x86,config=8888,"] = {1, 2, 3}
	//	[",arch=x86,config=565,"]  = {2, 4, 6}
	df := NewEmpty()
	traceids := []string{",arch=x86,config=8888,", ",arch=x86,config=565,"}
	df.TraceSet[traceids[0]] = types.Trace{1, 2, 3}
	df.TraceSet[traceids[1]] = types.Trace{2, 4, 6}
	df.Header = []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 3},
	}

	mockMetadataStore := traceStoreMocks.NewMetadataStore(t)

	config.Config = &config.InstanceConfig{
		DataPointConfig: config.DataPointConfig{
			KeysForUsefulLinks: []string{"link1", "link2", "link3", "link4", "link5", "link6"},
		},
	}

	sourceFileIds := []int64{
		12345,
		23456,
		34567,
		45678,
		56789,
		67890,
	}
	sInfo1 := types.NewTraceSourceInfo()
	sInfo1.Add(1, sourceFileIds[0])
	sInfo2 := types.NewTraceSourceInfo()
	sInfo2.Add(2, sourceFileIds[1])
	df.SourceInfo = map[string]*types.TraceSourceInfo{
		traceids[0]: sInfo1,
		traceids[1]: sInfo1,
	}

	sourceFileLinks := map[int64]map[string]string{
		sourceFileIds[0]: {
			"link1": "val1",
		},
		sourceFileIds[1]: {
			"link2": "val2",
		},
		sourceFileIds[2]: {
			"link3": "val3",
		},
		sourceFileIds[3]: {
			"link4": "val4",
		},
		sourceFileIds[4]: {
			"link5": "val5",
		},
		sourceFileIds[5]: {
			"link6": "val6",
		},
	}
	ctx := context.Background()
	mockMetadataStore.On("GetMetadataForSourceFileIDs", ctx, mock.Anything).Return(sourceFileLinks, nil)
	traceMetadata, err := GetMetadataForTraces(ctx, df, mockMetadataStore)
	assert.NoError(t, err)
	assert.NotNil(t, traceMetadata)
	assert.Equal(t, len(traceids), len(traceMetadata))
}
