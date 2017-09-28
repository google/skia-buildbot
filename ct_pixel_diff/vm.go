package main

import (
	"fmt"

	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func Prod() *gce.Instance {
	name := "skia-ct-pixel-diff"
	vm := server.Server20170928(name)
	vm.DataDisk.Name = fmt.Sprintf("%s-data", name)
	vm.DataDisk.SizeGb = 2000
	vm.DataDisk.Type = gce.DISK_TYPE_PERSISTENT_SSD
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_32
	vm.Metadata["auth_white_list"] = "google.com"
	vm.Metadata["owner_primary"] = "stephana"
	vm.Metadata["owner_secondary"] = "rmistry"
	return vm
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod": Prod(),
	})
}
