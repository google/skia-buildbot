package main

import (
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func PushBase(name, ipAddress string) *gce.Instance {
	vm := server.Server20170613(name)
	vm.DataDisk = nil
	vm.ExternalIpAddress = ipAddress
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_1
	vm.Metadata["owner_primary"] = "jcgregorio"

	vm.Scopes = append(vm.Scopes, auth.SCOPE_COMPUTE_READ_ONLY)
	return vm
}

func Prod() *gce.Instance {
	return PushBase("skia-push", "104.154.112.100")
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod": Prod(),
	})
}
