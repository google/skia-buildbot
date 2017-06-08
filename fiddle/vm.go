package main

import (
	"fmt"
	"io/ioutil"
	"path"
	"runtime"

	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/sklog"
)

const (
	SETUP_SCRIPT = "~/setup-script.sh"
)

func FiddleBase(name string) *gce.Instance {
	vm := server.Server20170518(name)
	vm.DataDisk.Name = fmt.Sprintf("%s-data", name)
	vm.DataDisk.SizeGb = 1000
	vm.DataDisk.Type = gce.DISK_TYPE_PERSISTENT_SSD
	vm.Gpu = true
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_8
	vm.MaintenancePolicy = gce.MAINTENANCE_POLICY_TERMINATE
	vm.Metadata["owner_primary"] = "jcgregorio"
	vm.ServiceAccount = "service-account-json@skia-buildbots.google.com.iam.gserviceaccount.com"

	// The setup script has to be run in an interactive terminal. Make sure
	// it ends up on the machine and we'll ask the user to run it.
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	setupContents, err := ioutil.ReadFile(path.Join(dir, "setup-script.sh"))
	if err != nil {
		sklog.Fatal(err)
	}
	vm.Metadata["setup-script"] = string(setupContents)
	vm.MetadataDownloads[SETUP_SCRIPT] = fmt.Sprintf(metadata.METADATA_URL, "instance", "setup-script")
	return vm
}

func Prod() *gce.Instance {
	return FiddleBase("skia-fiddle")
}

func main() {
	server.Main(gce.ZONE_GPU, map[string]*gce.Instance{
		"prod": Prod(),
	})
	sklog.Warningf("Instance created successfully. Please log in and run:\n$ %s", SETUP_SCRIPT)
}
