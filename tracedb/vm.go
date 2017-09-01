package main

import (
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func TraceDbBase(name string) *gce.Instance {
	vm := server.Server20170518(name)
	vm.DataDisk.SizeGb = 1000
	vm.DataDisk.Type = gce.DISK_TYPE_PERSISTENT_STANDARD
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_16
	vm.Metadata["owner_primary"] = "stephana"
	vm.Metadata["owner_secondary"] = "jcgregorio"
	return vm
}

func Prod() *gce.Instance {
	return TraceDbBase("skia-tracedb")
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod": Prod(),
	})
}
