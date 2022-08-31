// This empty package exists solely to appease //run_unittests.go, which runs "go test ./..." on
// any directories named "go" or "cmd" (see [1]). If the directory does not define any Go packages,
// the Go test runner fails with an error like this:
//
//     go: warning: "./..." matched no packages
//     no packages to test
//
// This file can be deleted once //run_unittests.go is removed.
//
// [1] https://skia.googlesource.com/buildbot/+/baae0ce503a9219f05f29cc9e1c1e4c224e5adba/run_unittests.go#290
package empty
