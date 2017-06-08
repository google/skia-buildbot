package main

import (
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func AndroidIngestBase(name, ipAddress string) *gce.Instance {
	vm := server.Server20170518(name)
	vm.DataDisk = nil
	vm.ExternalIpAddress = ipAddress
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_1
	vm.Metadata["owner_primary"] = "jcgregorio"
	vm.Metadata["owner_secondary"] = "stephana"
	vm.Scopes = append(vm.Scopes, auth.SCOPE_GERRIT)
	return vm
}

func Prod() *gce.Instance {
	return AndroidIngestBase("skia-android-ingest", "104.154.112.97")
}

func main() {
	server.Main(map[string]*gce.Instance{
		"prod": Prod(),
	})
}
