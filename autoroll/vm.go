package main

import (
	"path"
	"runtime"

	androidbuildinternal "go.skia.org/infra/go/androidbuildinternal/v2beta1"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func AutoRollBase(name, ipAddress string) *gce.Instance {
	vm := server.Server20170928(name)
	vm.DataDisks[0].SizeGb = 64
	vm.DataDisks[0].Type = gce.DISK_TYPE_PERSISTENT_STANDARD
	if ipAddress != "" {
		vm.ExternalIpAddress = ipAddress
	}
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_2
	vm.Metadata["owner_primary"] = "borenet"
	vm.Metadata["owner_secondary"] = "rmistry"
	return vm
}

func Skia() *gce.Instance {
	vm := AutoRollBase("skia-autoroll", "" /* Use ephemeral IP */)
	vm.ServiceAccount = "skia-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func SkiaInternal() *gce.Instance {
	vm := AutoRollBase("skia-internal-autoroll", "" /* Use ephemeral IP */)
	vm.ServiceAccount = "skia-internal-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	vm.SetupScript = path.Join(dir, "setup-script-internal.sh")
	return vm
}

func AngleSkia() *gce.Instance {
	vm := AutoRollBase("angle-skia-autoroll", "" /* Use ephemeral IP */)
	vm.ServiceAccount = "angle-skia-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func AngleChromium() *gce.Instance {
	vm := AutoRollBase("angle-chromium-autoroll", "" /* Use ephemeral IP */)
	vm.ServiceAccount = "angle-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func Catapult() *gce.Instance {
	vm := AutoRollBase("catapult-autoroll", "" /* Use ephemeral IP */)
	vm.ServiceAccount = "catapult-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func DepotTools_Chromium() *gce.Instance {
	vm := AutoRollBase("depot-tools-chromium-autoroll", "" /* Use ephemeral IP */)
	vm.ServiceAccount = "depot-tools-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func NaCl() *gce.Instance {
	vm := AutoRollBase("nacl-autoroll", "" /* Use ephemeral IP */)
	vm.ServiceAccount = "nacl-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func PDFium() *gce.Instance {
	vm := AutoRollBase("pdfium-autoroll", "" /* Use ephemeral IP */)
	vm.ServiceAccount = "pdfium-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func Fuchsia() *gce.Instance {
	vm := AutoRollBase("fuchsia-autoroll", "" /* Use ephemeral IP */)
	vm.ServiceAccount = "skia-fuchsia-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func SrcInternal_Chromium() *gce.Instance {
	vm := AutoRollBase("src-internal-chromium-autoroll", "" /* Use ephemeral IP */)
	vm.ServiceAccount = "src-internal-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func WebRTC_Chromium() *gce.Instance {
	vm := AutoRollBase("webrtc-chromium-autoroll", "" /* Use ephemeral IP */)
	vm.ServiceAccount = "webrtc-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func AddAndroidConfigs(vm *gce.Instance) *gce.Instance {
	vm.DataDisks[0].SizeGb = 512
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
	vm := server.Server20170928("google3-autoroll")
	vm.DataDisks[0].SizeGb = 64
	vm.DataDisks[0].Type = gce.DISK_TYPE_PERSISTENT_STANDARD
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_2
	vm.Metadata["owner_primary"] = "benjaminwagner"
	vm.Metadata["owner_secondary"] = "borenet"
	return vm
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"skia":                  Skia(),
		"skia-internal":         SkiaInternal(),
		"angle-chromium":        AngleChromium(),
		"angle-skia":            AngleSkia(),
		"catapult":              Catapult(),
		"depot-tools-chromium":  DepotTools_Chromium(),
		"google3":               Google3(),
		"nacl":                  NaCl(),
		"pdfium":                PDFium(),
		"fuchsia":               Fuchsia(),
		"android-master":        AndroidMaster(),
		"android-o":             AndroidO(),
		"src-internal-chromium": SrcInternal_Chromium(),
		"webrtc-chromium":       WebRTC_Chromium(),
	})
}
