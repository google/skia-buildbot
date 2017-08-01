package main

import (
	"fmt"
	"path"
	"runtime"

	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func CTPixelDiffBase(name, ipAddress string) *gce.Instance {
	vm := server.AddGitConfigs(server.Server20170613(name), "skia-ct-pixel-diff")
	vm.DataDisk.Name = fmt.Sprintf("%s-data", name)
	vm.DataDisk.SizeGb = 2000
	vm.DataDisk.Type = gce.DISK_TYPE_PERSISTENT_STANDARD
	vm.ExternalIpAddress = ipAddress
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_32
	vm.Metadata["auth_white_list"] = "google.com chromium.org skia.org"
	vm.Metadata["owner_primary"] = "lchoi"
	vm.Metadata["owner_secondary"] = "jcgregorio"

	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	vm.SetupScript = path.Join(dir, "setup-script.sh")
	return vm
}

func Prod() *gce.Instance {
	vm := CTPixelDiffBase("skia-ct-pixel-diff", "0.0.0.0")
	return vm
}

func DiffServer() *gce.Instance {
	vm := CTPixelDiffBase("skia-diffserver-prod", "0.0.0.0")
	delete(vm.Metadata, "auth_white_list")
	vm.DataDisk.Type = gce.DISK_TYPE_PERSISTENT_SSD
	return vm
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod":       Prod(),
		"diffserver": DiffServer(),
	})
}
