package main

import (
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func SwarmingLoggerBase(name string) *gce.Instance {
	vm := server.Server20170928(name)
	vm.DataDisks[0].SizeGb = 100
	vm.DataDisks[0].Type = gce.DISK_TYPE_PERSISTENT_STANDARD
	vm.Metadata["owner_primary"] = "borenet"
	vm.Metadata["owner_secondary"] = "benjaminwagner"
	return vm
}

func Prod() *gce.Instance {
	return SwarmingLoggerBase("skia-swarming-logger")
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod": Prod(),
	})
}
