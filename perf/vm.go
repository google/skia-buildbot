package main

import (
	"fmt"

	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func PerfBase(name, ipAddress string) *gce.Instance {
	vm := server.AddGitConfigs(server.Server20170518(name), "skia-perf")
	vm.DataDisk.Name = fmt.Sprintf("%s-ssd-data", name)
	vm.DataDisk.SizeGb = 1000
	vm.DataDisk.Type = gce.DISK_TYPE_PERSISTENT_SSD
	vm.ExternalIpAddress = ipAddress
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_32
	vm.Metadata["owner_primary"] = "jcgregorio"
	vm.Metadata["owner_secondary"] = "stephana"
	return vm
}

func Prod() *gce.Instance {
	return PerfBase("skia-perf", "104.154.112.108")
}

func AndroidMaster() *gce.Instance {
	return PerfBase("skia-android-master-perf", "104.154.112.139")
}

func Android() *gce.Instance {
	return PerfBase("skia-android-perf", "104.154.112.137")
}

func main() {
	server.Main(map[string]*gce.Instance{
		"prod":           Prod(),
		"android-master": AndroidMaster(),
		"android":        Android(),
	})
}
