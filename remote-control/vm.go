package main

import (
	"path"
	"runtime"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/vm_server"
)

func RemoteControlBase(name, ipAddress string) *gce.Instance {
	vm := vm_server.Server20170518(name, ipAddress)
	vm.DataDisk.SizeGb = 100
	vm.DataDisk.Type = gce.DISK_TYPE_PERSISTENT_STANDARD
	vm.Metadata["owner_primary"] = "benjaminwagner"
	vm.Metadata["owner_secondary"] = "TODO"
	vm.Scopes = append(vm.Scopes,
		auth.SCOPE_USERINFO_EMAIL,
		auth.SCOPE_USERINFO_PROFILE,
	)

	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	vm.StartupScript = path.Join(dir, "startup-script.sh")
	return vm
}

func RemoteControlProd() *gce.Instance {
	return RemoteControlBase("skia-remote-control", "130.211.145.124")
}

func RemoteControlTest() *gce.Instance {
	return RemoteControlBase("skia-remote-control-test", "35.184.141.175")
}

func main() {
	vm_server.Main(map[string]*gce.Instance{
		"prod": RemoteControlProd(),
		"test": RemoteControlTest(),
	})
}
