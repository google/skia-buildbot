package instance_types

import (
	"fmt"
	"path"
	"runtime"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gce"
	skia_instance_types "go.skia.org/infra/go/gce/swarming/instance_types"
)

const (
	GS_URL_GITCONFIG = "gs://cluster-telemetry-bucket/artifacts/bots/.gitconfig_ct"
	GS_URL_NETRC     = "gs://cluster-telemetry-bucket/artifacts/bots/.netrc_ct"
	GS_URL_BOTO      = "gs://cluster-telemetry-bucket/artifacts/bots/.boto_ct"

	CT_WORKER_PREFIX = "ct-gce-"

	WIN_SOURCE_IMAGE = "projects/windows-cloud/global/images/windows-server-2016-dc-v20190108"
)

// Base config for CT GCE instances.
func CT20170602(name string, useSSDDataDisk bool) *gce.Instance {
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(path.Dir(filename))
	dataDisk := &gce.Disk{
		Name:      fmt.Sprintf("%s-data", name),
		SizeGb:    300,
		Type:      gce.DISK_TYPE_PERSISTENT_STANDARD,
		MountPath: "/mnt/pd0",
	}
	if useSSDDataDisk {
		dataDisk.Type = gce.DISK_TYPE_PERSISTENT_SSD
	}
	return &gce.Instance{
		BootDisk: &gce.Disk{
			Name:        name,
			SourceImage: "ct-swarming-v1",
			Type:        gce.DISK_TYPE_PERSISTENT_STANDARD,
		},
		DataDisks: []*gce.Disk{dataDisk},
		GSDownloads: []*gce.GSDownload{
			{
				Source: GS_URL_GITCONFIG,
				Dest:   "/home/chrome-bot/.gitconfig",
			},
			{
				Source: GS_URL_NETRC,
				Dest:   "/home/chrome-bot/.netrc",
				Mode:   "600",
			},
			{
				Source: GS_URL_BOTO,
				Dest:   "/home/chrome-bot/.boto",
			},
		},
		MachineType:       gce.MACHINE_TYPE_HIGHMEM_4,
		Metadata:          map[string]string{},
		MetadataDownloads: map[string]string{},
		Name:              name,
		Os:                gce.OS_LINUX,
		ServiceAccount:    gce.SERVICE_ACCOUNT_CT_SWARMING,
		Scopes: []string{
			auth.SCOPE_FULL_CONTROL,
			auth.SCOPE_USERINFO_EMAIL,
			auth.SCOPE_PUBSUB,
		},
		SetupScript: path.Join(dir, "setup-script.sh"),
		Tags:        []string{"use-swarming-auth"},
		User:        gce.USER_CHROME_BOT,
	}
}

// CT GCE instances.
func CTInstance(num int) *gce.Instance {
	return CT20170602(fmt.Sprintf("%s%03d", CT_WORKER_PREFIX, num), false /* useSSDDataDisk */)
}

// CT Android Builder GCE instances.
func CTAndroidBuilderInstance(num int) *gce.Instance {
	vm := CT20170602(fmt.Sprintf("ct-android-builder-%03d", num), true /* useSSDDataDisk */)
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_64
	return vm
}

// CT Linux Builder GCE instances.
func CTLinuxBuilderInstance(num int) *gce.Instance {
	vm := CT20170602(fmt.Sprintf("ct-linux-builder-%03d", num), true /* useSSDDataDisk */)
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_64
	return vm
}

// CT Windows Builder GCE instances.
func CTWindowsBuilderInstance(num int, setupScriptPath, startupScriptPath, chromebotScript string) *gce.Instance {
	return getCTWindowsInstance(fmt.Sprintf("ct-windows-builder-%03d", num), gce.MACHINE_TYPE_HIGHMEM_64, gce.DISK_TYPE_PERSISTENT_SSD, setupScriptPath, startupScriptPath, chromebotScript)
}

// CT Windows GCE instances.
func CTWindowsInstance(num int, setupScriptPath, startupScriptPath, chromebotScript string) *gce.Instance {
	return getCTWindowsInstance(fmt.Sprintf("%s%03d", CT_WORKER_PREFIX, num), gce.MACHINE_TYPE_HIGHMEM_4, gce.DISK_TYPE_PERSISTENT_STANDARD, setupScriptPath, startupScriptPath, chromebotScript)
}

func getCTWindowsInstance(name, machineType, diskType, setupScriptPath, startupScriptPath, chromebotScript string) *gce.Instance {
	vm := skia_instance_types.Swarming20180406(name, machineType, gce.SERVICE_ACCOUNT_CT_SWARMING, setupScriptPath, WIN_SOURCE_IMAGE)
	return skia_instance_types.AddWinConfigs(vm, startupScriptPath, chromebotScript, diskType)
}
