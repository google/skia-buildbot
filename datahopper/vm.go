package main

import (
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func DatahopperBase(name string) *gce.Instance {
	vm := server.Server20170928(name)
	vm.DataDisk.SizeGb = 200
	vm.DataDisk.Type = gce.DISK_TYPE_PERSISTENT_SSD
	vm.Metadata["owner_primary"] = "borenet"
	vm.Metadata["owner_secondary"] = "jcgregorio"
	return vm
}

func Prod() *gce.Instance {
	return DatahopperBase("skia-datahopper2")
}

func Test1() *gce.Instance {
	return DatahopperBase("skia-datahopper-test1")
}

func Test2() *gce.Instance {
	return DatahopperBase("skia-datahopper-test2")
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod":  Prod(),
		"test1": Test1(),
		"test2": Test2(),
	})
}
