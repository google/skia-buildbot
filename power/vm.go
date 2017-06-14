package main

import (
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func PowerControllerInstance() *gce.Instance {
	vm := server.Server20170613("skia-power-controller")
	vm.Metadata["owner_primary"] = "kjlubick"
	vm.Metadata["owner_secondary"] = "stephana"
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_2
	return vm
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"power": PowerControllerInstance(),
	})
}
