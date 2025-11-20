package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/swarming/instance_types"
	instance_types_testing "go.skia.org/infra/go/gce/swarming/instance_types/testing"
)

// vmsToCreateTestCase checks only a subset of the gce.Instance struct fields, which should be
// enough to distinguish between the machine types created by swarming_vm.go.
type vmsToCreateTestCase struct {
	kind                         string
	instanceType                 string
	nums                         []int
	expectedZone                 string
	expectedProject              string
	expectedConfiguredViaAnsible bool
	expectedMachineType          string
	expectedSourceImage          string
	expectedServiceAccount       string
	expectedNames                []string
	expectedError                string
}

func testVMsToCreate(t *testing.T, tc vmsToCreateTestCase) {
	require.Equal(t, len(tc.nums), len(tc.expectedNames))

	// Windows machine creation shells out to GCloud to get the Skolo password.
	ctx := instance_types_testing.NewSecretsContextForTesting(t, context.Background())

	vmsToCreate, err := makeVMsToCreate(ctx, tc.kind, tc.instanceType, false /* =forceInstanceType */, tc.nums)

	if tc.expectedError != "" {
		require.Error(t, err)
		assert.Contains(t, err.Error(), tc.expectedError)
	} else {
		require.NoError(t, err)

		assert.Equal(t, tc.expectedZone, vmsToCreate.zone)
		assert.Equal(t, tc.expectedProject, vmsToCreate.project)
		assert.Equal(t, tc.expectedConfiguredViaAnsible, vmsToCreate.configuredViaAnsible)

		require.Len(t, vmsToCreate.vms, len(tc.expectedNames))
		for i, vm := range vmsToCreate.vms {
			t.Run(fmt.Sprintf("machine #%d", i), func(t *testing.T) {
				assert.Equal(t, tc.expectedNames[i], vm.Name)
				assert.Equal(t, tc.expectedMachineType, vm.MachineType)
				assert.Equal(t, tc.expectedSourceImage, vm.BootDisk.SourceImage)
				assert.Equal(t, tc.expectedServiceAccount, vm.ServiceAccount)
			})
		}
	}
}

func TestMakeVMsToCreate_External_0To99Range_AnyMachineTypeAllowed(t *testing.T) {
	t.Run("linux-micro", func(t *testing.T) {
		testVMsToCreate(t, vmsToCreateTestCase{
			kind:                         kindExternal,
			instanceType:                 instance_types.INSTANCE_TYPE_LINUX_MICRO,
			nums:                         []int{0, 1, 98, 99},
			expectedZone:                 "us-central1-c",
			expectedProject:              "skia-swarming-bots",
			expectedConfiguredViaAnsible: true,
			expectedMachineType:          "f1-micro",
			expectedSourceImage:          instance_types.DEBIAN_SOURCE_IMAGE_EXTERNAL,
			expectedServiceAccount:       gce.SERVICE_ACCOUNT_CHROMIUM_SWARM,
			expectedNames: []string{
				"skia-e-gce-000",
				"skia-e-gce-001",
				"skia-e-gce-098",
				"skia-e-gce-099",
			},
		})
	})

	t.Run("linux-small", func(t *testing.T) {
		testVMsToCreate(t, vmsToCreateTestCase{
			kind:                         kindExternal,
			instanceType:                 instance_types.INSTANCE_TYPE_LINUX_SMALL,
			nums:                         []int{0, 1, 98, 99},
			expectedZone:                 "us-central1-c",
			expectedProject:              "skia-swarming-bots",
			expectedConfiguredViaAnsible: true,
			expectedMachineType:          "n1-highmem-2",
			expectedSourceImage:          instance_types.DEBIAN_SOURCE_IMAGE_EXTERNAL,
			expectedServiceAccount:       gce.SERVICE_ACCOUNT_CHROMIUM_SWARM,
			expectedNames: []string{
				"skia-e-gce-000",
				"skia-e-gce-001",
				"skia-e-gce-098",
				"skia-e-gce-099",
			},
		})
	})

	t.Run("win-medium", func(t *testing.T) {
		testVMsToCreate(t, vmsToCreateTestCase{
			kind:                         kindExternal,
			instanceType:                 instance_types.INSTANCE_TYPE_WIN_MEDIUM,
			nums:                         []int{0, 1, 98, 99},
			expectedZone:                 "us-central1-c",
			expectedProject:              "skia-swarming-bots",
			expectedConfiguredViaAnsible: true,
			expectedMachineType:          "n1-standard-16",
			expectedSourceImage:          instance_types.WIN_SOURCE_IMAGE,
			expectedServiceAccount:       gce.SERVICE_ACCOUNT_CHROMIUM_SWARM,
			expectedNames: []string{
				"skia-e-gce-000",
				"skia-e-gce-001",
				"skia-e-gce-098",
				"skia-e-gce-099",
			},
		})
	})
}

func TestMakeVMsToCreate_External_100To199Range_LinuxSmall_Success(t *testing.T) {
	testVMsToCreate(t, vmsToCreateTestCase{
		kind:                         kindExternal,
		instanceType:                 instance_types.INSTANCE_TYPE_LINUX_SMALL,
		nums:                         []int{100, 101, 198, 199},
		expectedZone:                 "us-central1-c",
		expectedProject:              "skia-swarming-bots",
		expectedConfiguredViaAnsible: true,
		expectedMachineType:          "n1-highmem-2",
		expectedSourceImage:          instance_types.DEBIAN_SOURCE_IMAGE_EXTERNAL,
		expectedServiceAccount:       gce.SERVICE_ACCOUNT_CHROMIUM_SWARM,
		expectedNames: []string{
			"skia-e-gce-100",
			"skia-e-gce-101",
			"skia-e-gce-198",
			"skia-e-gce-199",
		},
	})
}

func TestMakeVMsToCreate_External_200To299Range_LinuxMedium_Success(t *testing.T) {
	testVMsToCreate(t, vmsToCreateTestCase{
		kind:                         kindExternal,
		instanceType:                 instance_types.INSTANCE_TYPE_LINUX_MEDIUM,
		nums:                         []int{200, 201, 298, 299},
		expectedZone:                 "us-central1-c",
		expectedProject:              "skia-swarming-bots",
		expectedConfiguredViaAnsible: true,
		expectedMachineType:          "n1-standard-16",
		expectedSourceImage:          instance_types.DEBIAN_SOURCE_IMAGE_EXTERNAL,
		expectedServiceAccount:       gce.SERVICE_ACCOUNT_CHROMIUM_SWARM,
		expectedNames: []string{
			"skia-e-gce-200",
			"skia-e-gce-201",
			"skia-e-gce-298",
			"skia-e-gce-299",
		},
	})
}

func TestMakeVMsToCreate_External_300To399Range_LinuxLarge_Success(t *testing.T) {
	testVMsToCreate(t, vmsToCreateTestCase{
		kind:                         kindExternal,
		instanceType:                 instance_types.INSTANCE_TYPE_LINUX_LARGE,
		nums:                         []int{300, 301, 398, 399},
		expectedZone:                 "us-central1-c",
		expectedProject:              "skia-swarming-bots",
		expectedConfiguredViaAnsible: true,
		expectedMachineType:          "n1-highcpu-64",
		expectedSourceImage:          instance_types.DEBIAN_SOURCE_IMAGE_EXTERNAL,
		expectedServiceAccount:       gce.SERVICE_ACCOUNT_CHROMIUM_SWARM,
		expectedNames: []string{
			"skia-e-gce-300",
			"skia-e-gce-301",
			"skia-e-gce-398",
			"skia-e-gce-399",
		},
	})
}

func TestMakeVMsToCreate_External_400To404Range_LinuxSkylake_Success(t *testing.T) {
	testVMsToCreate(t, vmsToCreateTestCase{
		kind:                         kindExternal,
		instanceType:                 instance_types.INSTANCE_TYPE_LINUX_SKYLAKE,
		nums:                         []int{400, 401, 402, 403, 404},
		expectedZone:                 "us-central1-b",
		expectedProject:              "skia-swarming-bots",
		expectedConfiguredViaAnsible: true,
		expectedMachineType:          "n1-standard-16",
		expectedSourceImage:          instance_types.DEBIAN_SOURCE_IMAGE_EXTERNAL,
		expectedServiceAccount:       gce.SERVICE_ACCOUNT_CHROMIUM_SWARM,
		expectedNames: []string{
			"skia-e-gce-400",
			"skia-e-gce-401",
			"skia-e-gce-402",
			"skia-e-gce-403",
			"skia-e-gce-404",
		},
	})
}

func TestMakeVMsToCreate_External_405To408Range_LinuxAmd_Success(t *testing.T) {
	testVMsToCreate(t, vmsToCreateTestCase{
		kind:                         kindExternal,
		instanceType:                 instance_types.INSTANCE_TYPE_LINUX_AMD,
		nums:                         []int{405, 406, 407, 408},
		expectedZone:                 "us-central1-a",
		expectedProject:              "skia-swarming-bots",
		expectedConfiguredViaAnsible: true,
		expectedMachineType:          "n2d-standard-16",
		expectedSourceImage:          instance_types.DEBIAN_SOURCE_IMAGE_EXTERNAL,
		expectedServiceAccount:       gce.SERVICE_ACCOUNT_CHROMIUM_SWARM,
		expectedNames: []string{
			"skia-e-gce-405",
			"skia-e-gce-406",
			"skia-e-gce-407",
			"skia-e-gce-408",
		},
	})
}

func TestMakeVMsToCreate_External_409To499Range_LinuxSkylake_Success(t *testing.T) {
	testVMsToCreate(t, vmsToCreateTestCase{
		kind:                         kindExternal,
		instanceType:                 instance_types.INSTANCE_TYPE_LINUX_SKYLAKE,
		nums:                         []int{409, 410, 498, 499},
		expectedZone:                 "us-central1-b",
		expectedProject:              "skia-swarming-bots",
		expectedConfiguredViaAnsible: true,
		expectedMachineType:          "n1-standard-16",
		expectedSourceImage:          instance_types.DEBIAN_SOURCE_IMAGE_EXTERNAL,
		expectedServiceAccount:       gce.SERVICE_ACCOUNT_CHROMIUM_SWARM,
		expectedNames: []string{
			"skia-e-gce-409",
			"skia-e-gce-410",
			"skia-e-gce-498",
			"skia-e-gce-499",
		},
	})
}

func TestMakeVMsToCreate_External_500To599Range_WinMedium_Success(t *testing.T) {
	testVMsToCreate(t, vmsToCreateTestCase{
		kind:                         kindExternal,
		instanceType:                 instance_types.INSTANCE_TYPE_WIN_MEDIUM,
		nums:                         []int{500, 501, 598, 599},
		expectedZone:                 "us-central1-c",
		expectedProject:              "skia-swarming-bots",
		expectedConfiguredViaAnsible: true,
		expectedMachineType:          "n1-standard-16",
		expectedSourceImage:          instance_types.WIN_SOURCE_IMAGE,
		expectedServiceAccount:       gce.SERVICE_ACCOUNT_CHROMIUM_SWARM,
		expectedNames: []string{
			"skia-e-gce-500",
			"skia-e-gce-501",
			"skia-e-gce-598",
			"skia-e-gce-599",
		},
	})
}

func TestMakeVMsToCreate_External_600To699Range_WinLarge_Success(t *testing.T) {
	testVMsToCreate(t, vmsToCreateTestCase{
		kind:                         kindExternal,
		instanceType:                 instance_types.INSTANCE_TYPE_WIN_LARGE,
		nums:                         []int{600, 601, 698, 699},
		expectedZone:                 "us-central1-c",
		expectedProject:              "skia-swarming-bots",
		expectedConfiguredViaAnsible: true,
		expectedMachineType:          "n1-highcpu-64",
		expectedSourceImage:          instance_types.WIN_SOURCE_IMAGE,
		expectedServiceAccount:       gce.SERVICE_ACCOUNT_CHROMIUM_SWARM,
		expectedNames: []string{
			"skia-e-gce-600",
			"skia-e-gce-601",
			"skia-e-gce-698",
			"skia-e-gce-699",
		},
	})
}

func TestMakeVMsToCreate_Internal_100To199Range_LinuxSmall_Success(t *testing.T) {
	testVMsToCreate(t, vmsToCreateTestCase{
		kind:                         kindInternal,
		instanceType:                 instance_types.INSTANCE_TYPE_LINUX_SMALL,
		nums:                         []int{100, 101, 198, 199},
		expectedZone:                 "us-central1-c",
		expectedProject:              "skia-swarming-bots-internal",
		expectedConfiguredViaAnsible: true,
		expectedMachineType:          "n1-highmem-2",
		expectedSourceImage:          instance_types.DEBIAN_SOURCE_IMAGE_INTERNAL,
		expectedServiceAccount:       gce.SERVICE_ACCOUNT_CHROME_SWARMING,
		expectedNames: []string{
			"skia-i-gce-100",
			"skia-i-gce-101",
			"skia-i-gce-198",
			"skia-i-gce-199",
		},
	})
}

func TestMakeVMsToCreate_Internal_200To299Range_LinuxLarge_Success(t *testing.T) {
	testVMsToCreate(t, vmsToCreateTestCase{
		kind:                         kindInternal,
		instanceType:                 instance_types.INSTANCE_TYPE_LINUX_LARGE,
		nums:                         []int{200, 201, 298, 299},
		expectedZone:                 "us-central1-c",
		expectedProject:              "skia-swarming-bots-internal",
		expectedConfiguredViaAnsible: true,
		expectedMachineType:          "n1-highcpu-64",
		expectedSourceImage:          instance_types.DEBIAN_SOURCE_IMAGE_INTERNAL,
		expectedServiceAccount:       gce.SERVICE_ACCOUNT_CHROME_SWARMING,
		expectedNames: []string{
			"skia-i-gce-200",
			"skia-i-gce-201",
			"skia-i-gce-298",
			"skia-i-gce-299",
		},
	})
}

func TestMakeVMsToCreate_Dev_100_LinuxSmall_Success(t *testing.T) {
	testVMsToCreate(t, vmsToCreateTestCase{
		kind:                         kindDev,
		instanceType:                 instance_types.INSTANCE_TYPE_LINUX_SMALL,
		nums:                         []int{100},
		expectedZone:                 "us-central1-c",
		expectedProject:              "skia-swarming-bots",
		expectedConfiguredViaAnsible: true,
		expectedMachineType:          "n1-highmem-2",
		expectedSourceImage:          instance_types.DEBIAN_SOURCE_IMAGE_EXTERNAL,
		expectedServiceAccount:       gce.SERVICE_ACCOUNT_CHROMIUM_SWARM,
		expectedNames: []string{
			"skia-d-gce-100",
		},
	})
}

func TestMakeVMsToCreate_Dev_101To599Range_LinuxMicro_Success(t *testing.T) {
	testVMsToCreate(t, vmsToCreateTestCase{
		kind:                         kindDev,
		instanceType:                 instance_types.INSTANCE_TYPE_LINUX_MICRO,
		nums:                         []int{101, 199, 200, 299, 300, 399, 400, 499, 500, 599},
		expectedZone:                 "us-central1-c",
		expectedProject:              "skia-swarming-bots",
		expectedConfiguredViaAnsible: true,
		expectedMachineType:          "f1-micro",
		expectedSourceImage:          instance_types.DEBIAN_SOURCE_IMAGE_EXTERNAL,
		expectedServiceAccount:       gce.SERVICE_ACCOUNT_CHROMIUM_SWARM,
		expectedNames: []string{
			"skia-d-gce-101",
			"skia-d-gce-199",
			"skia-d-gce-200",
			"skia-d-gce-299",
			"skia-d-gce-300",
			"skia-d-gce-399",
			"skia-d-gce-400",
			"skia-d-gce-499",
			"skia-d-gce-500",
			"skia-d-gce-599",
		},
	})
}

func TestMakeVMsToCreate_ForceInstanceType_Success(t *testing.T) {
	const (
		wrongInstanceTypeErr = "is expected to be of instance type"
		wrongNumberErr       = "is not in any known machine range"
	)

	for _, forceInstanceType := range []bool{false, true} {
		test := func(num int, kind, instanceType, expectedErrorWhenNotForced string) {
			name := fmt.Sprintf("%d, %s, ", num, kind)
			if forceInstanceType {
				name += "force instance type, success"
			} else {
				name += "do not force instance type, error"
			}
			t.Run(name, func(t *testing.T) {
				_, err := makeVMsToCreate(context.Background(), kind, instanceType, forceInstanceType, []int{num})
				if forceInstanceType {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					assert.Contains(t, err.Error(), expectedErrorWhenNotForced)
				}
			})
		}

		// skia-e-gce-* instances.
		//
		// We don't test range 0-99 because any machine type is allowed.
		test(100, kindExternal, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(199, kindExternal, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(200, kindExternal, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(299, kindExternal, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(300, kindExternal, instance_types.INSTANCE_TYPE_LINUX_MEDIUM, wrongInstanceTypeErr)
		test(399, kindExternal, instance_types.INSTANCE_TYPE_LINUX_MEDIUM, wrongInstanceTypeErr)
		test(400, kindExternal, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(404, kindExternal, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(405, kindExternal, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(408, kindExternal, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(409, kindExternal, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(499, kindExternal, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(509, kindExternal, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(599, kindExternal, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(609, kindExternal, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(699, kindExternal, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(700, kindExternal, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongNumberErr)
		test(799, kindExternal, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongNumberErr)
		test(800, kindExternal, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongNumberErr)
		test(899, kindExternal, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongNumberErr)
		test(900, kindExternal, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongNumberErr)
		test(999, kindExternal, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongNumberErr)

		// skia-i-gce-* instances.
		test(0, kindInternal, instance_types.INSTANCE_TYPE_LINUX_MICRO, wrongNumberErr)
		test(99, kindInternal, instance_types.INSTANCE_TYPE_LINUX_MICRO, wrongNumberErr)
		test(100, kindInternal, instance_types.INSTANCE_TYPE_LINUX_MICRO, wrongInstanceTypeErr)
		test(199, kindInternal, instance_types.INSTANCE_TYPE_LINUX_MICRO, wrongInstanceTypeErr)
		test(200, kindInternal, instance_types.INSTANCE_TYPE_LINUX_MICRO, wrongInstanceTypeErr)
		test(299, kindInternal, instance_types.INSTANCE_TYPE_LINUX_MICRO, wrongInstanceTypeErr)
		test(300, kindInternal, instance_types.INSTANCE_TYPE_LINUX_MICRO, wrongNumberErr)
		test(399, kindInternal, instance_types.INSTANCE_TYPE_LINUX_MICRO, wrongNumberErr)
		test(400, kindInternal, instance_types.INSTANCE_TYPE_LINUX_MICRO, wrongNumberErr)
		test(499, kindInternal, instance_types.INSTANCE_TYPE_LINUX_MICRO, wrongNumberErr)
		test(500, kindInternal, instance_types.INSTANCE_TYPE_LINUX_MICRO, wrongNumberErr)
		test(599, kindInternal, instance_types.INSTANCE_TYPE_LINUX_MICRO, wrongNumberErr)
		test(600, kindInternal, instance_types.INSTANCE_TYPE_LINUX_MICRO, wrongNumberErr)
		test(699, kindInternal, instance_types.INSTANCE_TYPE_LINUX_MICRO, wrongNumberErr)
		test(700, kindInternal, instance_types.INSTANCE_TYPE_LINUX_MICRO, wrongNumberErr)
		test(799, kindInternal, instance_types.INSTANCE_TYPE_LINUX_MICRO, wrongNumberErr)
		test(800, kindInternal, instance_types.INSTANCE_TYPE_LINUX_MICRO, wrongNumberErr)
		test(899, kindInternal, instance_types.INSTANCE_TYPE_LINUX_MICRO, wrongNumberErr)
		test(900, kindInternal, instance_types.INSTANCE_TYPE_LINUX_MICRO, wrongNumberErr)
		test(999, kindInternal, instance_types.INSTANCE_TYPE_LINUX_MICRO, wrongNumberErr)

		// skia-d-gce-* instances.
		test(0, kindDev, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongNumberErr)
		test(99, kindDev, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongNumberErr)
		test(100, kindDev, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(101, kindDev, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(199, kindDev, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(200, kindDev, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(299, kindDev, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(300, kindDev, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(399, kindDev, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(400, kindDev, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(499, kindDev, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(500, kindDev, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(599, kindDev, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongInstanceTypeErr)
		test(600, kindDev, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongNumberErr)
		test(699, kindDev, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongNumberErr)
		test(700, kindDev, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongNumberErr)
		test(799, kindDev, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongNumberErr)
		test(800, kindDev, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongNumberErr)
		test(899, kindDev, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongNumberErr)
		test(900, kindDev, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongNumberErr)
		test(999, kindDev, instance_types.INSTANCE_TYPE_LINUX_LARGE, wrongNumberErr)
	}
}
