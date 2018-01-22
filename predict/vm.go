package main

import (
	"fmt"

	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func PredictBase(name string) *gce.Instance {
	vm := server.Server20170928(name)
	vm.DataDisks[0].Name = fmt.Sprintf("%s-data", name)
	vm.DataDisks[0].SizeGb = 50
	vm.DataDisks[0].Type = gce.DISK_TYPE_PERSISTENT_STANDARD
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_1
	vm.Metadata["owner_primary"] = "jcgregorio"
	return vm
}

func Prod() *gce.Instance {
	return PredictBase("skia-predict")
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod": Prod(),
	})
}
