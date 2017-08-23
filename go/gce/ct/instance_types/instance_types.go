package instance_types

import (
	"fmt"
	"path"
	"runtime"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gce"
)

const (
	GS_URL_GITCONFIG = "gs://skia-buildbots/artifacts/bots/.gitconfig_ct"
	GS_URL_NETRC     = "gs://skia-buildbots/artifacts/bots/.netrc_ct"
	GS_URL_BOTO      = "gs://skia-buildbots/artifacts/bots/.boto_ct"

	CT_WORKER_PREFIX = "ct-gce-"
)

// Base config for CT GCE instances.
func CT20170602(name string) *gce.Instance {
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(path.Dir(filename))
	return &gce.Instance{
		BootDisk: &gce.Disk{
			Name:        name,
			SourceImage: "skia-swarming-v3",
			Type:        gce.DISK_TYPE_PERSISTENT_STANDARD,
		},
		DataDisk: &gce.Disk{
			Name:   fmt.Sprintf("%s-data", name),
			SizeGb: 300,
			Type:   gce.DISK_TYPE_PERSISTENT_STANDARD,
		},
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
		MachineType:       gce.MACHINE_TYPE_HIGHMEM_2,
		Metadata:          map[string]string{},
		MetadataDownloads: map[string]string{},
		Name:              name,
		Os:                gce.OS_LINUX,
		ServiceAccount:    gce.SERVICE_ACCOUNT_CHROME_SWARMING,
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
	return CT20170602(fmt.Sprintf("%s%03d", CT_WORKER_PREFIX, num))
}

// CT Android Builder GCE instances.
func CTAndroidBuilderInstance(num int) *gce.Instance {
	vm := CT20170602(fmt.Sprintf("ct-android-builder-%03d", num))
	vm.MachineType = "custom-32-70400"
	return vm
}

// CT Linux Builder GCE instances.
func CTLinuxBuilderInstance(num int) *gce.Instance {
	vm := CT20170602(fmt.Sprintf("ct-linux-builder-%03d", num))
	vm.MachineType = "custom-32-70400"
	return vm
}
