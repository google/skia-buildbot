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

func FlutterEngine_Flutter() *gce.Instance {
	vm := AutoRollBase("flutter-engine-flutter-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"rmistry@google.com",
	}
	vm.ServiceAccount = "engine-flutter-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
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
		"afdo-chromium":          AFDOChromium(),
		"android-master":         AndroidMaster(),
		"android-next":           AndroidNext(),
		"android-o":              AndroidO(),
		"flutter-engine-flutter": FlutterEngine_Flutter(),
		"fuchsia":                Fuchsia(),
		"fuchsia-sdk-chromium":   FuchsiaSDK_Chromium(),
		"google3":                Google3(),
		"skcms-skia":             SkCMS_Skia(),
		"skia-flutter":           Skia_Flutter(),
		"skia-internal":          SkiaInternal(),
		"skia-lottie-ci":         Skia_LottieCI(),
	})
}
