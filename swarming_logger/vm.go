package main

import (
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func SwarmingLoggerBase(name, ipAddress string) *gce.Instance {
	vm := server.Server20170518(name)
	vm.DataDisk.SizeGb = 100
	vm.DataDisk.Type = gce.DISK_TYPE_PERSISTENT_STANDARD
	vm.ExternalIpAddress = ipAddress
	vm.Metadata["owner_primary"] = "borenet"
	vm.Metadata["owner_secondary"] = "benjaminwagner"
	return vm
}

func Prod() *gce.Instance {
	return SwarmingLoggerBase("skia-swarming-logger", "104.154.112.140")
}

func main() {
	server.Main(map[string]*gce.Instance{
		"prod": Prod(),
	})
}
