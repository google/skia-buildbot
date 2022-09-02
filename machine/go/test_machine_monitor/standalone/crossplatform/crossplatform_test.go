package crossplatform

import (
	"testing"

	"github.com/shirou/gopsutil/host"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertIsaAndBitnessEqual(t *testing.T, arch, expectedIsa, expectedBitness string) {
	isa, bitness, err := isaAndBitness(arch)
	require.NoError(t, err)
	assert.Equal(t, expectedIsa, isa)
	assert.Equal(t, expectedBitness, bitness)
}

func TestIsaAndBitness(t *testing.T) {
	assertIsaAndBitnessEqual(t, "x86_64", "x86", "64")
	assertIsaAndBitnessEqual(t, "amd64", "x86", "64")
	assertIsaAndBitnessEqual(t, "aarch64", "arm64", "64")

	_, _, err := isaAndBitness("kersmoo")
	assert.Error(t, err, "An unknown CPU architecture should result in an error (and we should add it to the mapping).")
}

func assertCPUModelEqual(t *testing.T, vendor, brandString, expected, failureMessage string) {
	assert.Equal(t, expected, cpuModel(vendor, brandString), failureMessage)
}

func TestCPUModel(t *testing.T) {
	assertCPUModelEqual(
		t,
		"GenuineIntel",
		"Intel(R) Core(TM) i7-9750H v2 CPU @ 2.60GHz",
		"i7-9750H v2",
		"Intel model numbers should be extracted.",
	)
	assertCPUModelEqual(
		t,
		"Wackadoo Inc.",
		"Wackadoo ALU i5-9600",
		"Wackadoo_ALU_i5-9600",
		"Non-Intel model numbers should be smooshed into snake_case.",
	)
	assertCPUModelEqual(
		t,
		"GenuineIntel",
		"something it fails to extract anything from",
		"",
		"An unrecognizable Intel brand string should result in no extracted model.",
	)
	assertCPUModelEqual(
		t,
		"",
		"Wackadoo ALU",
		"Wackadoo_ALU",
		"An unrecognizable Intel brand string should result in no extracted model.",
	)
}

func TestCPUs(t *testing.T) {

	arch, err := host.KernelArch()
	require.NoError(t, err)
	isa, bitness, err := isaAndBitness(arch)
	require.NoError(t, err)
	vendor := "GenuineIntel"
	brandString := "Intel(R) Core(TM) i7-9750H v2 CPU @ 2.60GHz"
	model := cpuModel(vendor, brandString)

	dimensions, err := CPUs(vendor, brandString)
	require.NoError(t, err)
	assert.Equal(
		t,
		[]string{isa, isa + "-" + bitness, isa + "-" + bitness + "-" + model},
		dimensions,
		"If a model can be extracted, there should be a third slice element containing it.",
	)
	dimensions, err = CPUs("", "")
	require.NoError(t, err)
	assert.Equal(
		t,
		[]string{isa, isa + "-" + bitness},
		dimensions,
		"Empty vendor and brand string should result in no third element.",
	)
}

func TestVersionsOfAllPrecisions(t *testing.T) {
	assert.Equal(t, []string{"Mac", "Mac-12"}, VersionsOfAllPrecisions("Mac", "12"))
	assert.Equal(t, []string{"Mac", "Mac-12", "Mac-12.4"}, VersionsOfAllPrecisions("Mac", "12.4"))
	assert.Equal(t, []string{"Mac", "Mac-12", "Mac-12.4", "Mac-12.4.35"}, VersionsOfAllPrecisions("Mac", "12.4.35"))
	assert.Equal(t, []string{"Win"}, VersionsOfAllPrecisions("Win", ""))
}
