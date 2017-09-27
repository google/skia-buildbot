package main

import (
	"path"
	"runtime"

	androidbuildinternal "go.skia.org/infra/go/androidbuildinternal/v2beta1"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func AutoRollBase(name, ipAddress string) *gce.Instance {
	vm := server.SetGitCredsReadWrite(server.Server20170613(name), name)
	vm.DataDisk.SizeGb = 64
	vm.DataDisk.Type = gce.DISK_TYPE_PERSISTENT_STANDARD
	if ipAddress != "" {
		vm.ExternalIpAddress = ipAddress
	}
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_2
	vm.Metadata["owner_primary"] = "borenet"
	vm.Metadata["owner_secondary"] = "rmistry"
	vm.Scopes = append(vm.Scopes,
		auth.SCOPE_GERRIT,
	)
	return vm
}

func Skia() *gce.Instance {
	return AutoRollBase("skia-autoroll", "" /* Use ephemeral IP */)
}

func SkiaInternal() *gce.Instance {
	vm := AutoRollBase("skia-internal-autoroll", "" /* Use ephemeral IP */)
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	vm.SetupScript = path.Join(dir, "setup-script-internal.sh")
	return vm
}

func Angle() *gce.Instance {
	return AutoRollBase("angle-autoroll", "" /* Use ephemeral IP */)
}

func Catapult() *gce.Instance {
	return AutoRollBase("catapult-autoroll", "" /* Use ephemeral IP */)
}

func DepotTools_Chromium() *gce.Instance {
	return AutoRollBase("depot-tools-chromium-autoroll", "" /* Use ephemeral IP */)
}

func NaCl() *gce.Instance {
	return AutoRollBase("nacl-autoroll", "" /* Use ephemeral IP */)
}

func PDFium() *gce.Instance {
	return AutoRollBase("pdfium-autoroll", "" /* Use ephemeral IP */)
}

func Fuchsia() *gce.Instance {
	return AutoRollBase("fuchsia-autoroll", "" /* Use ephemeral IP */)
}

func AddAndroidConfigs(vm *gce.Instance) *gce.Instance {
	vm.DataDisk.SizeGb = 512
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_16
	vm.Scopes = append(vm.Scopes, androidbuildinternal.AndroidbuildInternalScope)

	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	vm.SetupScript = path.Join(dir, "setup-script-android.sh")
	return vm
}

func AndroidMaster() *gce.Instance {
	return AddAndroidConfigs(AutoRollBase("android-master-autoroll", "130.211.199.63" /* Needs whitelisted static IP */))
}

func AndroidO() *gce.Instance {
	return AddAndroidConfigs(AutoRollBase("android-o-autoroll", "104.198.73.244" /* Needs whitelisted static IP */))
}

func Google3() *gce.Instance {
	// Not using AutoRollBase because this server does not need auth.SCOPE_GERRIT.
	vm := server.Server20170912("google3-autoroll")
	vm.DataDisk.SizeGb = 64
	vm.DataDisk.Type = gce.DISK_TYPE_PERSISTENT_STANDARD
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_2
	vm.Metadata["owner_primary"] = "benjaminwagner"
	vm.Metadata["owner_secondary"] = "borenet"
	return vm
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"skia":                 Skia(),
		"skia-internal":        SkiaInternal(),
		"angle":                Angle(),
		"catapult":             Catapult(),
		"depot-tools-chromium": DepotTools_Chromium(),
		"google3":              Google3(),
		"nacl":                 NaCl(),
		"pdfium":               PDFium(),
		"fuchsia":              Fuchsia(),
		"android-master":       AndroidMaster(),
		"android-o":            AndroidO(),
	})
}
