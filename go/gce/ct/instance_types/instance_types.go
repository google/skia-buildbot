package instance_types

import (
	_ "embed"
	"fmt"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/skerr"
)

const (
	GS_URL_GITCONFIG = "gs://cluster-telemetry-bucket/artifacts/bots/.gitconfig_ct"
	GS_URL_NETRC     = "gs://cluster-telemetry-bucket/artifacts/bots/.netrc_ct"
	GS_URL_BOTO      = "gs://cluster-telemetry-bucket/artifacts/bots/.boto_ct"

	CT_WORKER_PREFIX = "ct-gce-"

	LINUX_SOURCE_IMAGE = "projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20240717"
)

//go:embed setup-script.sh
var embeddedSetupScript string

// Base config for CT GCE instances.
func CT20170602(name string, useSSDDataDisk bool) (*gce.Instance, error) {
	dataDisk := &gce.Disk{
		Name:      fmt.Sprintf("%s-data", name),
		SizeGb:    300,
		Type:      gce.DISK_TYPE_PERSISTENT_STANDARD,
		MountPath: "/b",
	}
	if useSSDDataDisk {
		dataDisk.Type = gce.DISK_TYPE_PERSISTENT_SSD
	}
	return &gce.Instance{
		BootDisk: &gce.Disk{
			Name:        name,
			SizeGb:      50,
			SourceImage: LINUX_SOURCE_IMAGE,
			Type:        gce.DISK_TYPE_PERSISTENT_STANDARD,
		},
		DataDisks:         []*gce.Disk{dataDisk},
		MachineType:       gce.MACHINE_TYPE_HIGHMEM_4,
		Metadata:          map[string]string{},
		MetadataDownloads: map[string]string{},
		Name:              name,
		Os:                gce.OS_LINUX,
		ServiceAccount:    gce.SERVICE_ACCOUNT_CT_SWARMING,
		Scopes: []string{
			auth.ScopeFullControl,
			auth.ScopeUserinfoEmail,
			auth.ScopePubsub,
			auth.ScopeGerrit,
		},
		SetupScript: embeddedSetupScript,
		Tags:        []string{"use-swarming-auth"},
		User:        gce.USER_CHROME_BOT,
	}, nil
}

// CT GCE instances.
func CTWorkerInstance(num int) (*gce.Instance, error) {
	vm, err := CT20170602(fmt.Sprintf("%s%03d", CT_WORKER_PREFIX, num), false /* useSSDDataDisk */)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return vm, nil
}

func CTMasterInstance(num int) (*gce.Instance, error) {
	vm, err := CT20170602(fmt.Sprintf("ct-master-%03d", num), false /* useSSDDataDisk */)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return vm, nil
}

// CT Android Builder GCE instances.
func CTAndroidBuilderInstance(num int) (*gce.Instance, error) {
	vm, err := CT20170602(fmt.Sprintf("ct-android-builder-%03d", num), true /* useSSDDataDisk */)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_64
	return vm, nil
}

// CT Linux Builder GCE instances.
func CTLinuxBuilderInstance(num int) (*gce.Instance, error) {
	vm, err := CT20170602(fmt.Sprintf("ct-linux-builder-%03d", num), true /* useSSDDataDisk */)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_64
	return vm, nil
}
