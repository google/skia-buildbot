package main

import (
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func PushBase(name string) *gce.Instance {
	vm := server.Server20170928(name)
	vm.DataDisks = nil
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_1
	vm.Metadata["owner_primary"] = "jcgregorio"
	vm.ServiceAccount = "skia-push@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func Prod() *gce.Instance {
	return PushBase("skia-push")
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod": Prod(),
	})
}
