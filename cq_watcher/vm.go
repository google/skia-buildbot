package main

import (
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func CqWatcherBase(name, ipAddress string) *gce.Instance {
	vm := server.Server20170518(name)
	vm.DataDisk = nil
	vm.ExternalIpAddress = ipAddress
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_2
	vm.Metadata["owner_primary"] = "rmistry"
	vm.Metadata["owner_secondary"] = "borenet"
	return vm
}

func Prod() *gce.Instance {
	return CqWatcherBase("skia-cq-watcher", "104.154.112.115")
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod": Prod(),
	})
}
