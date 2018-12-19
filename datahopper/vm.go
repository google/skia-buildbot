package main

import (
	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func DatahopperBase(name string) *gce.Instance {
	vm := server.Server20170928(name)
	vm.DataDisks[0].SizeGb = 200
	vm.DataDisks[0].Type = gce.DISK_TYPE_PERSISTENT_SSD
	vm.Metadata["owner_primary"] = "borenet"
	vm.Metadata["owner_secondary"] = "jcgregorio"
	vm.Scopes = append(vm.Scopes, bigtable.Scope)
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
