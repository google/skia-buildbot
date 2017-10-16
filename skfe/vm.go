package main

import (
	"fmt"

	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func SKFEBase(name string) *gce.Instance {
	vm := server.Server20170928(name)
	vm.DataDisks = nil
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_4
	vm.Metadata["owner_primary"] = "stephana"
	vm.Metadata["owner_secondary"] = "jcgregorio"
	return vm
}

func Prod(num int) *gce.Instance {
	return SKFEBase(fmt.Sprintf("skia-skfe-%d", num))
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod-1": Prod(1),
		"prod-2": Prod(2),
	})
}
