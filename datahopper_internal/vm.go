package main

import (
	"go.skia.org/infra/go/androidbuildinternal/v2beta1"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func DatahopperInternalBase(name string) *gce.Instance {
	vm := server.Server20170911(name)
	vm.DataDisk.SizeGb = 50
	vm.DataDisk.Type = gce.DISK_TYPE_PERSISTENT_STANDARD
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_2
	vm.Metadata["owner_primary"] = "jcgregorio"
	vm.Metadata["owner_secondary"] = "benjaminwagner"
	vm.Scopes = append(vm.Scopes,
		androidbuildinternal.AndroidbuildInternalScope,
	)
	return vm
}

func Prod() *gce.Instance {
	return DatahopperInternalBase("skia-internal")
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod": Prod(),
	})
}
