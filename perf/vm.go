package main

import (
	"fmt"

	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func PerfBase(name string) *gce.Instance {
	vm := server.AddGitConfigs(server.Server20170613(name), "skia-perf")
	vm.DataDisk.Name = fmt.Sprintf("%s-ssd-data", name)
	vm.DataDisk.SizeGb = 1000
	vm.DataDisk.Type = gce.DISK_TYPE_PERSISTENT_SSD
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_32
	vm.Metadata["owner_primary"] = "jcgregorio"
	vm.Metadata["owner_secondary"] = "stephana"
	return vm
}

func Prod() *gce.Instance {
	return PerfBase("skia-perf")
}

func AndroidMaster() *gce.Instance {
	return PerfBase("skia-android-master-perf")
}

func Android() *gce.Instance {
	return PerfBase("skia-android-perf")
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod":           Prod(),
		"android-master": AndroidMaster(),
		"android":        Android(),
	})
}
