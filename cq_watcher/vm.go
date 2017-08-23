package main

import (
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func CqWatcherBase(name string) *gce.Instance {
	vm := server.Server20170613(name)
	vm.DataDisk = nil
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_2
	vm.Metadata["owner_primary"] = "rmistry"
	vm.Metadata["owner_secondary"] = "borenet"
	return vm
}

func Prod() *gce.Instance {
	return CqWatcherBase("skia-cq-watcher")
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod": Prod(),
	})
}
