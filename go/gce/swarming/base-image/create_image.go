package main

import (
	"context"
	"flag"
	"path"
	"runtime"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/sklog"
)

const (
	IMAGE_DESCRIPTION = "Base image for Skia Swarming bots."
	IMAGE_FAMILY      = "skia-swarming-base"
	INSTANCE_NAME     = "skia-swarming-base-maker"
	SETUP_SCRIPT      = "~/setup_script.sh"
)

var (
	// Flags.
	internal = flag.Bool("internal", false, "Whether to create an image for internal bots.")
)

func BaseConfig(serviceAccount string) *gce.Instance {
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)

	vm := &gce.Instance{
		BootDisk: &gce.Disk{
			Name:        INSTANCE_NAME,
			SourceImage: "projects/debian-cloud/global/images/debian-10-buster-v20200309",
			Type:        gce.DISK_TYPE_PERSISTENT_STANDARD,
		},
		MachineType:    gce.MACHINE_TYPE_STANDARD_4,
		Name:           INSTANCE_NAME,
		Os:             gce.OS_LINUX,
		Scopes:         []string{auth.ScopeAllCloudAPIs},
		ServiceAccount: serviceAccount,
		SetupScript:    path.Join(dir, "setup-script.sh"),
		User:           gce.USER_CHROME_BOT,
	}

	return vm
}

func main() {
	common.Init()

	// Create the GCloud object.
	project := gce.PROJECT_ID_SWARMING
	serviceAccount := gce.SERVICE_ACCOUNT_CHROMIUM_SWARM
	if *internal {
		project = gce.PROJECT_ID_INTERNAL_SWARMING
		serviceAccount = gce.SERVICE_ACCOUNT_CHROME_SWARMING
	}
	g, err := gce.NewLocalGCloud(project, gce.ZONE_DEFAULT)
	if err != nil {
		sklog.Fatal(err)
	}
	if err := g.CheckSsh(); err != nil {
		sklog.Fatal(err)
	}

	vm := BaseConfig(serviceAccount)

	// Delete the instance if it already exists, to ensure that we're in a
	// clean state.
	if err := g.Delete(vm, true, true); err != nil {
		sklog.Fatal(err)
	}

	// Create/Setup the instance.
	if err := g.CreateAndSetup(context.Background(), vm, false); err != nil {
		sklog.Fatal(err)
	}

	// Capture the image.
	if err := g.CaptureImage(vm, IMAGE_FAMILY, IMAGE_DESCRIPTION); err != nil {
		sklog.Fatal(err)
	}
}
