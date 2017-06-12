package main

import (
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func PowerControllerInstance() *gce.Instance {
	vm := server.Server20170518("us-central1-c", "skia-power-controller")
	vm.Metadata["owner_primary"] = "kjlubick"
	vm.Metadata["owner_secondary"] = "stephana"
	vm.Scopes = append(vm.Scopes,
		auth.SCOPE_USERINFO_EMAIL,
		auth.SCOPE_USERINFO_PROFILE,
	)
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_2
	return vm
}

func main() {
	server.Main(map[string]*gce.Instance{
		"power": PowerControllerInstance(),
	})
}
