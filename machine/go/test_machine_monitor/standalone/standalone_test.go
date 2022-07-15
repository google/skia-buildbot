package standalone

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestMacVersionsOfAllPrecisions(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, macVersionsOfAllPrecisions("12"), []string{"Mac", "Mac-12"})
	assert.Equal(t, macVersionsOfAllPrecisions("12.4"), []string{"Mac", "Mac-12", "Mac-12.4"})
	assert.Equal(t, macVersionsOfAllPrecisions("12.4.35"), []string{"Mac", "Mac-12", "Mac-12.4", "Mac-12.4.35"})
}

func assertCpuDimensions(t *testing.T, arch string, vendor string, brandString string, expected []string, failureMessage string) {
	dimensions, err := cpusCore(arch, vendor, brandString)
	assert.NoError(t, err)
	assert.Equal(t, expected, dimensions, failureMessage)
}

func TestCpusCore_ParsingAndBitWidthAndArchMapping(t *testing.T) {
	unittest.SmallTest(t)
	assertCpuDimensions(
		t,
		"x86_64",
		"GenuineIntel",
		"Intel(R) Core(TM) i7-9750H v2 CPU @ 2.60GHz",
		[]string{"x86", "x86-64", "x86-64-i7-9750H v2"},
		"x86_64 should be recognized as x86 ISA, and Intel model numbers should be extracted.",
	)
	assertCpuDimensions(
		t,
		"amd64",
		"Wackadoo Inc.",
		"Wackadoo ALU i5-9600",
		[]string{"x86", "x86-64", "x86-64-Wackadoo_ALU_i5-9600"},
		"amd64 should be recognized as x86 ISA, and non-Intel model numbers should be smooshed into snake_case.",
	)
	assertCpuDimensions(
		t,
		"aarch64",
		"GenuineIntel",
		"something it fails to extract anything from",
		[]string{"arm64", "arm64-64"},
		"aarch64 should be recognized as arm64 ISA, and an unrecognizable Intel brand string should result in no third element.",
	)
}

func TestCpusCore_UnrecognizedArch_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	_, err := cpusCore(
		"kersmoo",
		"GenuineIntel",
		"Intel(R) Core(TM) i7-9750H v2 CPU @ 2.60GHz",
	)
	assert.Error(t, err, "An unknown CPU architecture should result in an error (and we should add it to the mapping).")
}
