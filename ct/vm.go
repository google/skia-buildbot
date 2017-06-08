package main

import (
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func CtBase(name, ipAddress string) *gce.Instance {
	vm := server.Server20170518(name)
	vm.DataDisk = nil
	vm.ExternalIpAddress = ipAddress
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_2
	vm.Metadata["owner_primary"] = "rmistry"
	vm.Metadata["owner_secondary"] = "benjaminwagner"
	return vm
}

func Prod() *gce.Instance {
	return CtBase("skia-ctfe", "104.154.112.110")
}

func main() {
	server.Main(map[string]*gce.Instance{
		"prod": Prod(),
	})
}
