package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/sklog"
)

const (
	IMAGE_DESCRIPTION = "Base image for all Skia servers."
	IMAGE_FAMILY      = "skia-pushable-base"
	INSTANCE_NAME     = fmt.Sprintf("%s-maker", IMAGE_FAMILY)
	SETUP_SCRIPT      = "~/setup_script.sh"
)

var (
	// Flags.
	workdir = flag.String("workdir", ".", "Working directory.")
)

func BaseConfig() *gce.Instance {
	// The setup script has to be run in an interactive terminal. Make sure
	// it ends up on the machine and we'll ask the user to run it.
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	setupContents, err := ioutil.ReadFile(path.Join(dir, "setup-script.sh"))
	if err != nil {
		sklog.Fatal(err)
	}

	vm := &gce.Instance{
		BootDisk: &gce.Disk{
			Name:        INSTANCE_NAME,
			SourceImage: "projects/debian-cloud/global/images/debian-8-jessie-v20170523",
			Type:        gce.DISK_TYPE_PERSISTENT_STANDARD,
		},
		MachineType: gce.MACHINE_TYPE_STANDARD_4,
		Metadata: map[string]string{
			"setup-script": string(setupContents),
		},
		MetadataDownloads: map[string]string{
			SETUP_SCRIPT: fmt.Sprintf(metadata.METADATA_URL, "instance", "setup-script"),
		},
		Name: INSTANCE_NAME,
		Os:   gce.OS_LINUX,
		Scopes: []string{
			"https://www.googleapis.com/auth/cloud-platform",
		},
		User: gce.USER_DEFAULT,
	}

	return vm
}

func main() {
	common.Init()
	defer common.LogPanic()

	// Get the absolute workdir.
	wdAbs, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	// Create the GCloud object.
	g, err := gce.NewGCloud(gce.ZONE_DEFAULT, wdAbs)
	if err != nil {
		sklog.Fatal(err)
	}

	// Create/Setup the instance.
	vm := BaseConfig()
	if err := g.CreateAndSetup(vm, false); err != nil {
		sklog.Fatal(err)
	}

	// Pause, ask the user to run the setup script.
	sklog.Infof(`Please log in to the machine and run the setup script:

$ ssh -i ~/.ssh/google_compute_engine -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no %s@%s
$ chmod +x %s
$ %s

Then press enter to continue.`, vm.User, vm.ExternalIpAddress, SETUP_SCRIPT, SETUP_SCRIPT)
	bufio.NewReader(os.Stdin).ReadBytes('\n')

	// Capture the image.
	if err := g.CaptureImage(vm, IMAGE_FAMILY, IMAGE_DESCRIPTION); err != nil {
		sklog.Fatal(err)
	}
}
