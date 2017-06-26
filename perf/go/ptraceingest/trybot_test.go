package ptraceingest

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/ingestcommon"
	"go.skia.org/infra/perf/go/ptracestore"
)

// TestTrybotBenchData tests parsing and processing of a single trybot file.
func TestTrybotBenchData(t *testing.T) {
	testutils.SmallTest(t)
	// Load the sample data file as BenchData.
	r, err := os.Open(filepath.Join(TEST_DATA_DIR, "trybot.json"))
	assert.NoError(t, err)

	benchData, err := ingestcommon.ParseBenchDataFromReader(r)
	assert.NoError(t, err)
	traceSet := getValueMap(benchData)
	expected := map[string]float32{
		",arch=x86_64,bench_type=micro,compiler=Clang,config=gpu,cpu_or_gpu=GPU,cpu_or_gpu_value=GeForce320M,model=MacMini4.1,name=GLInstancedArraysBench_instance,os=Mac10.8,source_type=bench,sub_result=min_ms,test=GLInstancedArraysBench_instance_640_480,": 0.0052282223,
		",arch=x86_64,bench_type=micro,compiler=Clang,config=gpu,cpu_or_gpu=GPU,cpu_or_gpu_value=GeForce320M,model=MacMini4.1,name=GLInstancedArraysBench_one_0,os=Mac10.8,source_type=bench,sub_result=min_ms,test=GLInstancedArraysBench_one_0_640_480,":       7.122931e-06}
	testutils.AssertDeepEqual(t, expected, traceSet)
}

// Tests the processor in conjunction with Rietveld.
func TestPerfTrybotProcessor(t *testing.T) {
	testutils.MediumTest(t)
	orig := ptracestore.Default
	dir, err := ioutil.TempDir("", "ptrace")
	assert.NoError(t, err)
	ptracestore.Default, err = ptracestore.New(dir)
	assert.NoError(t, err)
	defer func() {
		ptracestore.Default = orig
		testutils.RemoveAll(t, dir)
	}()

	b, err := ioutil.ReadFile(filepath.Join("testdata", "rietveld_response.txt"))
	assert.NoError(t, err)
	m := mockhttpclient.NewURLMock()
	m.Mock("https://codereview.chromium.org/api/1467533002", mockhttpclient.MockGetDialogue(b))
	m.Mock("https://chromium-cq-status.appspot.com/v2/patch-summary/codereview.chromium.org/2320153002/840001", mockhttpclient.MockGetDialogue([]byte("{}")))

	ingesterConf := &sharedconfig.IngesterConfig{}
	processor, err := newPerfTrybotProcessor(nil, ingesterConf, nil)
	assert.NoError(t, err)

	processor.(*perfTrybotProcessor).review = rietveld.New("https://codereview.chromium.org", m.Client())

	fsResult, err := ingestion.FileSystemResult(filepath.Join(TEST_DATA_DIR, "trybot.json"), TEST_DATA_DIR)
	assert.NoError(t, err)
	err = processor.Process(fsResult)
	assert.NoError(t, err)

	traceId := ",arch=x86_64,bench_type=micro,compiler=Clang,config=gpu,cpu_or_gpu=GPU,cpu_or_gpu_value=GeForce320M,model=MacMini4.1,name=GLInstancedArraysBench_one_0,os=Mac10.8,source_type=bench,sub_result=min_ms,test=GLInstancedArraysBench_one_0_640_480,"
	expectedValue := float32(7.122931e-06)
	cid := &cid.CommitID{
		Source: "https://codereview.chromium.org/1467533002",
		Offset: 2,
	}
	source, value, err := ptracestore.Default.Details(cid, traceId)
	assert.NoError(t, err)
	assert.Equal(t, expectedValue, value)
	assert.Equal(t, "trybot.json", source)
}
