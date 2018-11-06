package main

import (
	"path"
	"runtime"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
	logging "google.golang.org/api/logging/v2"
)

func FuzzerBase(name string) *gce.Instance {
	vm := server.Server20170928(name)
	vm.BootDisk.Type = gce.DISK_TYPE_PERSISTENT_SSD
	vm.DataDisks[0].SizeGb = 375
	vm.DataDisks[0].Type = gce.DISK_TYPE_LOCAL_SSD
	vm.Metadata["owner_primary"] = "kjlubick"
	vm.Metadata["owner_secondary"] = "jcgregorio"
	vm.Scopes = append(vm.Scopes, auth.SCOPE_READ_WRITE, logging.LoggingWriteScope)
	vm.ServiceAccount = "fuzzer@skia-buildbots.google.com.iam.gserviceaccount.com"

	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	vm.SetupScript = path.Join(dir, "setup-script.sh")
	vm.StartupScript = path.Join(dir, "startup-script.sh")
	return vm
}

func FrontEnd() *gce.Instance {
	vm := FuzzerBase("skia-fuzzer-fe")
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_8
	return vm
}

func main() {
	common.Init()
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"fe": FrontEnd(),
	})
}
