package main

import (
	"fmt"
	"path"
	"runtime"

	"go.skia.org/infra/go/androidbuildinternal/v2beta1"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func GoldBase(name, ipAddress string) *gce.Instance {
	vm := server.AddGitConfigs(server.Server20170518(name), "skia-gold")
	vm.DataDisk.Name = fmt.Sprintf("%s-data", name)
	vm.DataDisk.SizeGb = 2000
	vm.DataDisk.Type = gce.DISK_TYPE_PERSISTENT_STANDARD
	vm.ExternalIpAddress = ipAddress
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_32
	vm.Metadata["auth_white_list"] = "google.com chromium.org skia.org"
	vm.Metadata["owner_primary"] = "stephana"
	vm.Metadata["owner_secondary"] = "jcgregorio"

	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	vm.SetupScript = path.Join(dir, "setup-script.sh")
	return vm
}

func Prod() *gce.Instance {
	vm := GoldBase("skia-gold-prod", "104.154.112.104")
	vm.Metadata["auth_white_list"] = `skia.org
kkinnunen@nvidia.com
mjk@nvidia.com
vbuzinov@nvidia.com
martina.kollarova@intel.com
this.is.harry.stern@gmail.com
dvonbeck@gmail.com
zakerinasab@chromium.org`
	return vm
}

func Pdfium() *gce.Instance {
	vm := GoldBase("skia-gold-pdfium", "104.154.112.106")
	vm.DataDisk.SizeGb = 500
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_16
	return vm
}

func Test() *gce.Instance {
	vm := GoldBase("skia-gold-testinstance", "104.154.112.111")
	vm.DataDisk.SizeGb = 500
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_16
	vm.Scopes = append(vm.Scopes, androidbuildinternal.AndroidbuildInternalScope)
	return vm
}

func DiffServer() *gce.Instance {
	vm := GoldBase("skia-diffserver-prod", "104.154.123.224")
	delete(vm.Metadata, "auth_white_list")
	vm.DataDisk.SizeGb = 2000
	vm.DataDisk.Type = gce.DISK_TYPE_PERSISTENT_SSD
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_32
	return vm
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod":         Prod(),
		"pdfium":       Pdfium(),
		"testinstance": Test(),
		"diffserver":   DiffServer(),
	})
}
