package main

import (
	"fmt"

	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func DocBase(name, ipAddress string) *gce.Instance {
	vm := server.Server20170613(name) // TODO(borenet): Needs git configs.
	vm.DataDisk.Name = fmt.Sprintf("%s-data", name)
	vm.DataDisk.SizeGb = 1000
	vm.DataDisk.Type = gce.DISK_TYPE_PERSISTENT_STANDARD
	vm.ExternalIpAddress = ipAddress
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_4
	vm.Metadata["owner_primary"] = "jcgregorio"
	return vm
}

func Prod() *gce.Instance {
	return DocBase("skia-docs", "104.154.112.101")
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod": Prod(),
	})
}
