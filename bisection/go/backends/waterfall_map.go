package backends

// Builds from waterfall can also be recycled for bisection
//
// As part of our anomaly detection system, Waterfall builders
// will continuously build Chrome near the tip of main. Sometimes,
// Pinpoint jobs will attempt to build a CL Waterfall has already
// built i.e. verifying a regression. These builds are automatic.
// Pinpoint builders will only build Chrome on demand. The Waterfall
// and Pinpoint builders are maintained in separate pools.
//
// The map is maintained here:
// https://chromium.googlesource.com/chromium/tools/build/+/986f23767a01508ad1eb39194ffdb5fec4f00d7b/recipes/recipes/pinpoint/builder.py#22
// TODO(b/316207255): move this builder map to a more stable config file
var PinpointWaterfall = map[string]string{
	"Android Compile Perf":                       "android-builder-perf",
	"Android Compile Perf PGO":                   "android-builder-perf-pgo",
	"Android arm64 Compile Perf":                 "android_arm64-builder-perf",
	"Android arm64 Compile Perf PGO":             "android_arm64-builder-perf-pgo",
	"Android arm64 High End Compile Perf":        "android_arm64_high_end-builder-perf",
	"Android arm64 High End Compile Perf PGO":    "android_arm64_high_end-builder-perf-pgo",
	"Chromecast Linux Builder Perf":              "chromecast-linux-builder-perf",
	"Chromeos Amd64 Generic Lacros Builder Perf": "chromeos-amd64-generic-lacros-builder-perf",
	"Fuchsia Builder Perf":                       "fuchsia-builder-perf-arm64",
	"Linux Builder Perf":                         "linux-builder-perf",
	"Linux Builder Perf PGO":                     "linux-builder-perf-pgo",
	"Mac Builder Perf":                           "mac-builder-perf",
	"Mac Builder Perf PGO":                       "mac-builder-perf-pgo",
	"Mac arm Builder Perf":                       "mac-arm-builder-perf",
	"Mac arm Builder Perf PGO":                   "mac-arm-builder-perf-pgo",
	"mac-laptop_high_end-perf":                   "mac-laptop_high_end-perf",
	"Win x64 Builder Perf":                       "win64-builder-perf",
	"Win x64 Builder Perf PGO":                   "win64-builder-perf-pgo",
}
