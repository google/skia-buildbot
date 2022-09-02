// Package pivot provides the ability to pivot dataframes.
package pivot

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/types"
)

const badValue = "not a valid choice"

func TestOptionsValid_Valid_ReturnsNil(t *testing.T) {
	assert.NoError(t, Request{
		GroupBy:   []string{"test"},
		Operation: Avg,
		Summary:   []Operation{Avg},
	}.Valid())
}

func TestOptionsValid_EmptyGroupBy_ReturnsError(t *testing.T) {
	assert.Contains(t, Request{
		GroupBy:   []string{},
		Operation: Avg,
	}.Valid().Error(), "GroupBy")
}

func TestOptionsValid_BadOperation_ReturnsError(t *testing.T) {

	assert.Contains(t, Request{
		GroupBy:   []string{"test"},
		Operation: Operation(badValue),
	}.Valid().Error(), badValue)
}

func TestOptionsValid_BadColumns_ReturnsError(t *testing.T) {

	assert.Contains(t, Request{
		GroupBy:   []string{"test"},
		Operation: Avg,
		Summary:   []Operation{badValue},
	}.Valid().Error(), badValue)
}

func TestIntermediateKeyFromFullKey_AllKeysExist_ReturnsCorrectKey(t *testing.T) {
	const traceKey = ",arch=arm,config=8888,"
	actual := groupKeyFromTraceKey(paramtools.NewParams(traceKey), []string{"arch", "config"})
	assert.Equal(t, traceKey, actual)
}

func TestIntermediateKeyFromFullKey_OnlySomeKeysAreSelected_UnselectedKeysAreRemoved(t *testing.T) {
	actual := groupKeyFromTraceKey(paramtools.NewParams(",arch=arm,config=8888,device=Nexus7"), []string{"arch", "config"})
	assert.Equal(t, ",arch=arm,config=8888,", actual)
}

func TestIntermediateKeyFromFullKey_OneKeyDoesNotExist_ReturnsEmptyString(t *testing.T) {
	actual := groupKeyFromTraceKey(paramtools.NewParams(",arch=arm,config=8888,"), []string{"unknown_key"})
	assert.Equal(t, "", actual)
}

func TestIntermediateKeyFromFullKey_NoKeys_ReturnsEmptyString(t *testing.T) {
	actual := groupKeyFromTraceKey(paramtools.NewParams(",arch=arm,config=8888,"), nil)
	assert.Equal(t, "", actual)
}

func dataframeForTesting() *dataframe.DataFrame {
	df := dataframe.NewEmpty()
	df.TraceSet = types.TraceSet{
		",arch=arm,config=8888,device=Nexus5,":   types.Trace{1, 0, 0},
		",arch=arm,config=565,device=Nexus5,":    types.Trace{0, 2, 0},
		",arch=arm,config=gles,device=Nexus5,":   types.Trace{0, 0, 3},
		",arch=intel,config=8888,device=Nexus5,": types.Trace{1, 2, 3},
		",arch=intel,config=565,device=Nexus5,":  types.Trace{1, 2, 3},
		",arch=intel,config=gles,device=Nexus5,": types.Trace{1, 2, 3},
		",arch=arm,config=8888,device=Nexus7,":   types.Trace{10, 0, 0},
		",arch=arm,config=565,device=Nexus7,":    types.Trace{0, 20, 0},
		",arch=arm,config=gles,device=Nexus7,":   types.Trace{0, 0, 30},
		",arch=intel,config=8888,device=Nexus7,": types.Trace{10, 20, 30},
		",arch=intel,config=565,device=Nexus7,":  types.Trace{10, 20, 30},
		",arch=intel,config=gles,device=Nexus7,": types.Trace{10, 20, 30},
		",this=trace,does=not,match=anykeys,":    types.Trace{100, 100, 100},
	}
	df.Header = []*dataframe.ColumnHeader{}
	for i := 0; i < 3; i++ {
		df.Header = append(df.Header, &dataframe.ColumnHeader{Offset: types.CommitNumber(i)})
	}

	df.BuildParamSet()
	return df
}

func TestPivot_InvalidRequest_ReturnsError(t *testing.T) {

	req := Request{
		GroupBy:   []string{},
		Operation: Operation("not-a-valid-operation"),
	}
	df := dataframeForTesting()
	_, err := Pivot(context.Background(), req, df)
	require.Error(t, err)
}

func TestPivot_KeyNotInParamSet_ReturnsError(t *testing.T) {

	req := Request{
		GroupBy:   []string{"unknown_key"},
		Operation: Sum,
	}
	df := dataframeForTesting()
	_, err := Pivot(context.Background(), req, df)
	require.Error(t, err)
}

func TestPivot_SumOperationNoSummary_Success(t *testing.T) {

	req := Request{
		GroupBy:   []string{"arch", "device"},
		Operation: Sum,
	}
	df := dataframeForTesting()
	df, err := Pivot(context.Background(), req, df)
	require.NoError(t, err)
	require.Equal(t, types.TraceSet{
		",arch=arm,device=Nexus5,":   types.Trace{1, 2, 3},
		",arch=intel,device=Nexus5,": types.Trace{3, 6, 9},
		",arch=arm,device=Nexus7,":   types.Trace{10, 20, 30},
		",arch=intel,device=Nexus7,": types.Trace{30, 60, 90},
	}, df.TraceSet)
	require.NotEmpty(t, df.ParamSet)
}

func TestPivot_SumOperationNoSummaryExtraKeyInParamSet_GroupsWithNoTracesAreMissingFromResult(t *testing.T) {

	req := Request{
		GroupBy:   []string{"arch"},
		Operation: Sum,
	}
	df := dataframeForTesting()

	// Add risc-v as an arch.
	df.ParamSet["arch"] = append(df.ParamSet["arch"], "risc-v")
	df, err := Pivot(context.Background(), req, df)
	require.NoError(t, err)

	// Note that risc-v does not appear in result.
	require.Equal(t, types.TraceSet{
		",arch=arm,":   types.Trace{11, 22, 33},
		",arch=intel,": types.Trace{33, 66, 99},
	}, df.TraceSet)
}

func TestPivot_SumOperationWithSummary_Success(t *testing.T) {

	req := Request{
		GroupBy:   []string{"arch", "device"},
		Operation: Sum,
		Summary:   []Operation{Avg, Sum},
	}
	df := dataframeForTesting()
	df, err := Pivot(context.Background(), req, df)
	require.NoError(t, err)
	require.Equal(t, types.TraceSet{
		",arch=arm,device=Nexus5,":   types.Trace{2, 6},
		",arch=intel,device=Nexus5,": types.Trace{6, 18},
		",arch=arm,device=Nexus7,":   types.Trace{20, 60},
		",arch=intel,device=Nexus7,": types.Trace{60, 180},
	}, df.TraceSet)
	require.NotEmpty(t, df.ParamSet)

}

func TestPivot_ContextIsCancelled_ReturnsError(t *testing.T) {

	req := Request{
		GroupBy:   []string{"arch", "device"},
		Operation: Sum,
		Summary:   []Operation{Avg, Sum},
	}
	df := dataframeForTesting()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Pivot(ctx, req, df)
	require.Contains(t, err.Error(), "canceled")
}
