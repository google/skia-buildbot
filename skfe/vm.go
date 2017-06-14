package main

import (
	"fmt"

	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func SKFEBase(name, ipAddress string) *gce.Instance {
	vm := server.Server20170613(name)
	vm.DataDisk = nil
	vm.ExternalIpAddress = ipAddress
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_4
	vm.Metadata["owner_primary"] = "stephana"
	vm.Metadata["owner_secondary"] = "jcgregorio"
	return vm
}

func Prod(num int, ip string) *gce.Instance {
	return SKFEBase(fmt.Sprintf("skia-skfe-%d", num), ip)
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod-1": Prod(1, "104.154.112.11"),
		"prod-2": Prod(2, "104.154.112.103"),
	})
}
