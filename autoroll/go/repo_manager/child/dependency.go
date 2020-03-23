package child

/*
Example:

  "child": {
    ...
    "dependencies": {
       "DEPS": [
         "https://skia.googlesource.com/skia.git",
       ],
       "file": {
         "fuchsia_sdk_linux": "build/fuchsia/linux.sdk.sha1",
         "fuchsia_sdk_mac": "build/fuchsia/mac.sdk.sha1"
       }
    }
  }
*/

// DependencyConfig describes which dependencies should be added to Revisions
// from the Child.
type DependencyConfig struct {
	Deps DepsDependencyConfig `json:"DEPS"`
	File FileDependencyConfig `json:"file"`
}

// DepsDependencyConfig describes dependencies obtained via DEPS.
type DepsDependencyConfig []string

// FileDependencyConfig describes dependencies whose versions are written as the
// entire contents of individual files.
type FileDependencyConfig map[string]string
