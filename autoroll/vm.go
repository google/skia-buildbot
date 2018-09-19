package main

import (
	"cloud.google.com/go/datastore"
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

func Skia_Flutter() *gce.Instance {
	vm := AutoRollBase("skia-flutter-autoroll", "" /* Use ephemeral IP */)
	vm.Contacts = []string{
		"rmistry@google.com",
		"brianosman@google.com",
	}
	vm.ServiceAccount = "skia-flutter-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
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
		"flutter-engine-flutter": FlutterEngine_Flutter(),
		"fuchsia":                Fuchsia(),
		"fuchsia-sdk-chromium":   FuchsiaSDK_Chromium(),
		"google3":                Google3(),
		"skia-flutter":           Skia_Flutter(),
	})
}
