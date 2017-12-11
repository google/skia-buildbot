package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/suggester/go/dsconst"
)

func TestCountsReadWrite(t *testing.T) {
	testutils.MediumTest(t)
	cleanup := testutil.InitDatastore(t, dsconst.FILE_COUNT)

	defer cleanup()

	totals := map[string]map[string]int{
		"bench/ColorCodecBench.cpp": map[string]int{
			"Test-Chromecast-GCC-Chorizo-CPU-Cortex_A7-arm-Release-All":             1,
			"Test-Chromecast-GCC-Chorizo-GPU-Cortex_A7-arm-Release-All":             1,
			"Test-Debian9-Clang-GCE-CPU-AVX2-x86_64-Debug-All-MSAN_FAAA":            1,
			"Test-Debian9-Clang-GCE-CPU-AVX2-x86_64-Debug-All-MSAN_FDAA":            1,
			"Test-Debian9-Clang-GCE-CPU-AVX2-x86_64-Debug-All-MSAN_FSAA":            1,
			"Test-Win10-Clang-NUC5i7RYH-GPU-IntelIris6100-x86_64-Debug-All-ANGLE":   1,
			"Test-Win10-Clang-NUC5i7RYH-GPU-IntelIris6100-x86_64-Release-All-ANGLE": 1,
			"Test-Win10-Clang-NUCD34010WYKH-GPU-IntelHD4400-x86_64-Debug-All-ANGLE": 1,
		},
	}

	err := WriteTotals(totals)
	assert.NoError(t, err)
	time.Sleep(2)
	readTotals, err := ReadTotals()
	assert.NoError(t, err)
	assert.Len(t, readTotals, 1)
}
