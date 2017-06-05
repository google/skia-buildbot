package main

import (
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/vm_server"
)

func PowerControllerInstance() *gce.Instance {
	vm := vm_server.Server20170518("skia-power-controller", "")
	vm.Metadata["owner_primary"] = "kjlubick"
	vm.Metadata["owner_secondary"] = "stephana"
	vm.Scopes = append(vm.Scopes,
		auth.SCOPE_USERINFO_EMAIL,
		auth.SCOPE_USERINFO_PROFILE,
	)
	return vm
}

func main() {
	vm_server.Main(map[string]*gce.Instance{
		"power": PowerControllerInstance(),
	})
}
