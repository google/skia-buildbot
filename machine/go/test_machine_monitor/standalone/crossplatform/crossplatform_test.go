package crossplatform

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
)

func TestISAAndBitness(t *testing.T) {
	test := func(arch, expectedISA, expectedBitness string) {
		t.Run(arch, func(t *testing.T) {
			isa, bitness, err := isaAndBitness(arch)
			require.NoError(t, err)
			assert.Equal(t, expectedISA, isa)
			assert.Equal(t, expectedBitness, bitness)
		})
	}

	// Spot-check a few 64-bit architectures. 32-bit architectures are tricky because the function
	// under test uses strconv.IntSize to determine bitness.
	test("x86_64", "x86", "64")
	test("arm64", "arm64", "64")
	test("aarch64", "arm64", "64")

	_, _, err := isaAndBitness("unknown architecture")
	require.Error(t, err)
}

func TestCPUModel(t *testing.T) {
	test := func(name, vendor, brandString, expected string) {
		t.Run(name, func(t *testing.T) {
			actual := cpuModel(vendor, brandString)
			assert.Equal(t, expected, actual)
		})
	}

	test(
		"Intel model numbers are extracted",
		"GenuineIntel",
		"Intel(R) Core(TM) i7-9750H v2 CPU @ 2.60GHz",
		"i7-9750H v2")

	test(
		"Newer Intel model numbers are extracted",
		"GenuineIntel",
		"11th Gen Intel(R) Core(TM) i5-1135G7 @ 2.40GHz",
		"i5-1135G7")

	test(
		"Non-Intel vendors result in the brand string",
		"Wackadoo Inc.",
		"Wackadoo ALU i5-9600",
		"Wackadoo ALU i5-9600")

	test(
		"An unrecognizable Intel brand string results in no extracted model",
		"GenuineIntel",
		"unrecognizable brand string",
		"")

	test(
		"An empty vendor results in the brand string",
		/* vendor= */ "",
		"Wackadoo ALU",
		"Wackadoo ALU",
	)
}

func TestCPUs(t *testing.T) {
	test := func(name, vendor, brandString, gceCPUPlatform string, expected []string) {
		t.Run(name, func(t *testing.T) {
			// Set up mocks.
			mockHostKernelArch(t, func() (string, error) { return "x86_64", nil })
			if gceCPUPlatform == "" {
				mockMetadataOnGCE(t, false)
			} else {
				mockMetadataOnGCE(t, true)
				mockMetadataGet(t, func(suffix string) (string, error) {
					require.Equal(t, suffix, "instance/cpu-platform")
					return gceCPUPlatform, nil
				})
			}

			actual, err := CPUs(vendor, brandString)
			require.NoError(t, err)
			assert.Equal(t, expected, actual)
		})
	}

	test(
		"If a model can be extracted, there should be a third slice element containing it",
		"GenuineIntel",
		"Intel(R) Core(TM) i7-9750H v2 CPU @ 2.60GHz",
		/* gceCPUPlatform= */ "",
		[]string{"x86", "x86-64", "x86-64-i7-9750H_v2"})

	test(
		"Empty vendor and brand string should result in no third slice element",
		/* vendor= */ "",
		/* brandString= */ "",
		/* gceCPUPlatform= */ "",
		[]string{"x86", "x86-64"})

	test(
		"Intel Haswell GCE example based on skia-e-gce-300 on 2023-05-09",
		"GenuineIntel",
		"Intel(R) Xeon(R) CPU @ 2.30GHz",
		"Intel Haswell",
		[]string{"x86", "x86-64", "x86-64-Haswell_GCE"})

	test(
		"Intel Skylake GCE example based on skia-e-gce-400 on 2023-05-09",
		"GenuineIntel",
		"Intel(R) Xeon(R) CPU @ 2.00GHz",
		"Intel Skylake",
		[]string{"x86", "x86-64", "x86-64-Skylake_GCE"})

	test(
		"AMD GCE example based on skia-e-gce-405 on 2023-05-09",
		"AuthenticAMD",
		"AMD EPYC 7B12",
		"AMD Rome",
		[]string{"x86", "x86-64", "x86-64-AMD_Rome_GCE"})

	test(
		"AMD GCE example based on skia-e-win-260 on 2023-08-22",
		"AuthenticAMD",
		"AMD Ryzen 5 4500U with Radeon Graphics",
		/* gceCPUPlatform= */ "",
		[]string{"x86", "x86-64", "x86-64-AMD_Ryzen_5_4500U_with_Radeon_Graphics"})

}

func TestVersionsOfAllPrecisions(t *testing.T) {
	assert.Equal(t, []string{"Mac", "Mac-12"}, VersionsOfAllPrecisions("Mac", "12"))
	assert.Equal(t, []string{"Mac", "Mac-12", "Mac-12.4"}, VersionsOfAllPrecisions("Mac", "12.4"))
	assert.Equal(t, []string{"Mac", "Mac-12", "Mac-12.4", "Mac-12.4.35"}, VersionsOfAllPrecisions("Mac", "12.4.35"))
	assert.Equal(t, []string{"Win"}, VersionsOfAllPrecisions("Win", ""))
}

func TestGCEMachineType_OnGCE_ReturnsAndCachesLastPartOfMachineType(t *testing.T) {
	alreadyCalled := false

	mockMetadataOnGCE(t, true)
	mockMetadataGet(t, func(suffix string) (string, error) {
		require.False(t, alreadyCalled)
		require.Equal(t, "instance/machine-type", suffix)
		return "projects/123456789/machineTypes/n1-standard-16", nil
	})

	// Test with an empty cache.
	cachedMachineType = ""
	machineType, err := GCEMachineType()
	require.NoError(t, err)
	assert.Equal(t, "n1-standard-16", machineType)

	// Once the cache is populated, metadata.Get should not be called again.
	alreadyCalled = true
	machineType, err = GCEMachineType()
	require.NoError(t, err)
	assert.Equal(t, "n1-standard-16", machineType)
}

func TestGCEMachineType_NotOnGCE_ReturnsEmptyString(t *testing.T) {
	mockMetadataOnGCE(t, false)

	cachedMachineType = ""
	machineType, err := GCEMachineType()
	require.NoError(t, err)
	assert.Empty(t, machineType)
}

func TestIsDockerInstalled_Installed_ReturnsTrue(t *testing.T) {
	ctx := exec.NewContext(context.Background(), func(ctx context.Context, cmd *exec.Command) error {
		require.Equal(t, "docker", cmd.Name)
		require.Equal(t, []string{"version"}, cmd.Args)
		cmd.CombinedOutput = bytes.NewBufferString("non-empty output ignored by the function under test")
		return nil
	})

	assert.True(t, IsDockerInstalled(ctx))
}

func TestIsDockerInstalled_NotInstalled_ReturnsFalse(t *testing.T) {
	ctx := exec.NewContext(context.Background(), func(ctx context.Context, cmd *exec.Command) error {
		require.Equal(t, "docker", cmd.Name)
		require.Equal(t, []string{"version"}, cmd.Args)
		return fmt.Errorf("the specific error does not matter for this test")
	})

	assert.False(t, IsDockerInstalled(ctx))
}

func mockHostKernelArch(t *testing.T, fn func() (string, error)) {
	original := hostKernelArch
	t.Cleanup(func() { hostKernelArch = original })
	hostKernelArch = fn
}

func mockMetadataOnGCE(t *testing.T, onGCE bool) {
	original := metadataOnGCE
	t.Cleanup(func() { metadataOnGCE = original })
	metadataOnGCE = func() bool { return onGCE }
}

func mockMetadataGet(t *testing.T, fn func(suffix string) (string, error)) {
	original := metadataGet
	t.Cleanup(func() { metadataGet = original })
	metadataGet = fn
}
