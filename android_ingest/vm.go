package main

import (
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func AndroidIngestBase(name string) *gce.Instance {
	vm := server.Server20170928(name)
	vm.DataDisks = nil
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_1
	vm.Metadata["owner_primary"] = "jcgregorio"
	vm.Metadata["owner_secondary"] = "stephana"
	vm.Scopes = append(vm.Scopes, auth.SCOPE_GERRIT)
	return vm
}

func Prod() *gce.Instance {
	return AndroidIngestBase("skia-android-ingest")
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod": Prod(),
	})
}
