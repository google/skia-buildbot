package main

import (
	"path"
	"runtime"

	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func StatusBase(name string) *gce.Instance {
	vm := server.AddGitConfigs(server.Server20170613(name), name)
	vm.DataDisk.SizeGb = 100
	vm.DataDisk.Type = gce.DISK_TYPE_PERSISTENT_STANDARD
	vm.Metadata["owner_primary"] = "borenet"
	vm.Metadata["owner_secondary"] = "kjlubick"

	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	vm.StartupScript = path.Join(dir, "startup-script.sh")
	return vm
}

func StatusProd() *gce.Instance {
	return StatusBase("skia-status")
}

func StatusInternal() *gce.Instance {
	return StatusBase("skia-status-internal")
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod":     StatusProd(),
		"internal": StatusInternal(),
	})
}
