package main

import (
	"path"
	"runtime"

	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func ImageInfoBase(name, ipAddress string) *gce.Instance {
	vm := server.Server20170613(name)
	vm.DataDisk.SizeGb = 1000
	vm.DataDisk.Type = gce.DISK_TYPE_PERSISTENT_STANDARD
	vm.ExternalIpAddress = ipAddress
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_4
	vm.Metadata["owner_primary"] = "jcgregorio"

	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	vm.SetupScript = path.Join(dir, "setup-script.sh")
	return vm
}

func Prod() *gce.Instance {
	return ImageInfoBase("skia-imageinfo", "104.154.112.127")
}

func Test() *gce.Instance {
	return ImageInfoBase("borenet-vm-creation-test", "")
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod": Prod(),
		"test": Test(),
	})
}
