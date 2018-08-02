package main

import (
	"path"
	"runtime"

	"cloud.google.com/go/datastore"
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
	vm.Scopes = append(vm.Scopes, datastore.ScopeDatastore)
	return vm
}

func Skia() *gce.Instance {
	vm := AutoRollBase("skia-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"borenet@google.com",
	}
	vm.ServiceAccount = "skia-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func SkiaInternal() *gce.Instance {
	vm := AutoRollBase("skia-internal-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"borenet@google.com",
	}
	vm.ServiceAccount = "skia-internal-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	vm.SetupScript = path.Join(dir, "setup-script-internal.sh")
	return vm
}

func AFDOChromium() *gce.Instance {
	vm := AutoRollBase("afdo-chromium-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"gbiv@chromium.org",
	}
	vm.ServiceAccount = "afdo-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func AngleSkia() *gce.Instance {
	vm := AutoRollBase("angle-skia-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"jmadill@google.com",
	}
	vm.ServiceAccount = "angle-skia-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func AngleChromium() *gce.Instance {
	vm := AutoRollBase("angle-chromium-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"jmadill@google.com",
	}
	vm.ServiceAccount = "angle-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func Catapult() *gce.Instance {
	vm := AutoRollBase("catapult-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"sullivan@google.com",
	}
	vm.ServiceAccount = "catapult-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func Chromite_Chromium() *gce.Instance {
	vm := AutoRollBase("chromite-chromium-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"bpastene@google.com",
	}
	vm.ServiceAccount = "chromite-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func Chromium_Skia() *gce.Instance {
	vm := AutoRollBase("chromium-skia-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"borenet@google.com",
	}
	vm.ServiceAccount = "chromium-skia-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func DepotTools_Chromium() *gce.Instance {
	vm := AutoRollBase("depot-tools-chromium-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"agable@google.com",
	}
	vm.ServiceAccount = "depot-tools-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func IosInternal_Chromium() *gce.Instance {
	vm := AutoRollBase("ios-internal-chromium-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"sdefresne@google.com",
	}
	vm.ServiceAccount = "ios-internal-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func LottieWeb_LottieCI() *gce.Instance {
	vm := AutoRollBase("lottie-web-lottie-ci-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"borenet@google.com",
	}
	vm.ServiceAccount = "lottie-web-lottie-ci-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func NaCl() *gce.Instance {
	vm := AutoRollBase("nacl-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"mseaborn@google.com",
	}
	vm.ServiceAccount = "nacl-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func PDFium() *gce.Instance {
	vm := AutoRollBase("pdfium-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"dsinclair@google.com",
	}
	vm.ServiceAccount = "pdfium-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func PerfettoChromium() *gce.Instance {
	vm := AutoRollBase("perfetto-chromium-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"primiano@chromium.org",
	}
	vm.ServiceAccount = "perfetto-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func Fuchsia() *gce.Instance {
	vm := AutoRollBase("fuchsia-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"brianosman@google.com",
		"rmistry@google.com",
	}
	vm.ServiceAccount = "skia-fuchsia-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func FuchsiaSDK_Chromium() *gce.Instance {
	vm := AutoRollBase("fuchsia-sdk-chromium-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"jbudorick@chromium.org",
		"cr-fuchsia+bot@chromium.org",
	}
	vm.ServiceAccount = "fuchsia-sdk-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func SkCMS_Skia() *gce.Instance {
	vm := AutoRollBase("skcms-skia-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"brianosman@google.com",
		"mtklein@google.com",
	}
	vm.ServiceAccount = "skcms-skia-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func Skia_Flutter() *gce.Instance {
	vm := AutoRollBase("skia-flutter-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"rmistry@google.com",
		"brianosman@google.com",
	}
	vm.ServiceAccount = "skia-flutter-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func Skia_LottieCI() *gce.Instance {
	vm := AutoRollBase("skia-lottie-ci-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"borenet@google.com",
	}
	vm.ServiceAccount = "skia-lottie-ci-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	// This is not an internal roller, but it needs the same setup.
	vm.SetupScript = path.Join(dir, "setup-script-internal.sh")
	return vm
}

func SrcInternal_Chromium() *gce.Instance {
	vm := AutoRollBase("src-internal-chromium-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"mmoss@google.com",
	}
	vm.ServiceAccount = "src-internal-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func SwiftShader_Skia() *gce.Instance {
	vm := AutoRollBase("swiftshader-skia-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"benjaminwagner@google.com",
		"halcanary@google.com",
	}
	vm.ServiceAccount = "swiftshader-skia-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func WebRTC_Chromium() *gce.Instance {
	vm := AutoRollBase("webrtc-chromium-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"comms-engprod-sto@google.com",
	}
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
	vm := AutoRollBase("android-master-autoroll", "130.211.199.63" /* Needs whitelisted static IP */)
	vm.Contacts = []string{
		"djsollen@google.com",
		"rmistry@google.com",
	}
	return AddAndroidConfigs(vm)
}

func AndroidNext() *gce.Instance {
	vm := AutoRollBase("android-next-autoroll", "35.202.27.169" /* Needs whitelisted static IP */)
	vm.Contacts = []string{
		"djsollen@google.com",
		"rmistry@google.com",
	}
	return AddAndroidConfigs(vm)
}

func AndroidO() *gce.Instance {
	vm := AutoRollBase("android-o-autoroll", "104.198.73.244" /* Needs whitelisted static IP */)
	vm.Contacts = []string{
		"djsollen@google.com",
		"rmistry@google.com",
	}
	return AddAndroidConfigs(vm)
}

func Google3() *gce.Instance {
	// Not using AutoRollBase because this server does not need auth.SCOPE_GERRIT.
	vm := server.Server20170928("google3-autoroll")
	vm.Contacts = []string{
		"benjaminwagner@google.com",
	}
	vm.DataDisks[0].SizeGb = 64
	vm.DataDisks[0].Type = gce.DISK_TYPE_PERSISTENT_STANDARD
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_2
	vm.Metadata["owner_primary"] = "benjaminwagner"
	vm.Metadata["owner_secondary"] = "borenet"
	return vm
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"afdo-chromium":         AFDOChromium(),
		"android-master":        AndroidMaster(),
		"android-next":          AndroidNext(),
		"android-o":             AndroidO(),
		"angle-chromium":        AngleChromium(),
		"angle-skia":            AngleSkia(),
		"catapult":              Catapult(),
		"chromite-chromium":     Chromite_Chromium(),
		"chromium-skia":         Chromium_Skia(),
		"depot-tools-chromium":  DepotTools_Chromium(),
		"fuchsia":               Fuchsia(),
		"fuchsia-sdk-chromium":  FuchsiaSDK_Chromium(),
		"google3":               Google3(),
		"ios-internal-chromium": IosInternal_Chromium(),
		"lottie-web-lottie-ci":  LottieWeb_LottieCI(),
		"nacl":                  NaCl(),
		"pdfium":                PDFium(),
		"perfetto-chromium":     PerfettoChromium(),
		"skcms-skia":            SkCMS_Skia(),
		"skia":                  Skia(),
		"skia-flutter":          Skia_Flutter(),
		"skia-internal":         SkiaInternal(),
		"skia-lottie-ci":        Skia_LottieCI(),
		"src-internal-chromium": SrcInternal_Chromium(),
		"swiftshader-skia":      SwiftShader_Skia(),
		"webrtc-chromium":       WebRTC_Chromium(),
	})
}
