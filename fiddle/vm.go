package main

import (
	"fmt"
	"path"
	"runtime"

	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

const (
	SETUP_SCRIPT = "~/setup-script.sh"
)

func FiddleBase(name string) *gce.Instance {
	vm := server.Server20170928(name)
	vm.DataDisks[0].Name = fmt.Sprintf("%s-data", name)
	vm.DataDisks[0].SizeGb = 1000
	vm.DataDisks[0].Type = gce.DISK_TYPE_PERSISTENT_SSD
	vm.Gpu = true
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_8
	vm.MaintenancePolicy = gce.MAINTENANCE_POLICY_TERMINATE
	vm.Metadata["owner_primary"] = "jcgregorio"
	vm.ServiceAccount = "service-account-json@skia-buildbots.google.com.iam.gserviceaccount.com"

	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	vm.SetupScript = path.Join(dir, "setup-script.sh")
	return vm
}

func Prod() *gce.Instance {
	return FiddleBase("skia-fiddle")
}

func main() {
	server.Main(gce.ZONE_GPU, map[string]*gce.Instance{
		"prod": Prod(),
	})
}
