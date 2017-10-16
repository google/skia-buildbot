package main

import (
	"fmt"

	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func PerfBase(name, ipAddress string) *gce.Instance {
	vm := server.Server20170928(name)
	vm.DataDisks[0].Name = fmt.Sprintf("%s-ssd-data", name)
	vm.DataDisks[0].SizeGb = 1000
	vm.DataDisks[0].Type = gce.DISK_TYPE_PERSISTENT_SSD
	vm.ExternalIpAddress = ipAddress
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_32
	vm.Metadata["owner_primary"] = "jcgregorio"
	vm.Metadata["owner_secondary"] = "stephana"
	return vm
}

func Prod() *gce.Instance {
	return PerfBase("skia-perf", "35.192.9.78" /* Whitelisted in skiaperf cloud DB */)
}

func AndroidMaster() *gce.Instance {
	return PerfBase("skia-android-master-perf", "35.202.218.36" /* Whitelisted in skiaperf cloud DB */)
}

func Android() *gce.Instance {
	return PerfBase("skia-android-perf", "104.198.232.107" /* Whitelisted in skiaperf cloud DB */)
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod":           Prod(),
		"android-master": AndroidMaster(),
		"android":        Android(),
	})
}
