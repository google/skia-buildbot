// Sample app to demonstrate how to use buildskia.
package main

import (
	"os"
	"path/filepath"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/buildskia"
	"go.skia.org/infra/go/common"
)

func main() {
	common.Init()
	path := "/tmp/buildcmake/"
	if err := os.MkdirAll(path, 0777); err != nil {
		glog.Fatalf("Failed to create tmp dir: %s", err)
	}
	if _, err := buildskia.DownloadSkia("e7ec417268d4be2d7921b23c131859b322badf78", path, "/usr/local/google/home/jcgregorio/projects/depot_tools", false, false); err != nil {
		glog.Fatalf("Failed to fetch: %s", err)
	}
	glog.Info("Starting CMakeBuild")
	if err := buildskia.CMakeBuild(path, buildskia.RELEASE_BUILD); err != nil {
		glog.Fatalf("Failed cmake build: %s", err)
	}
	glog.Info("Starting NinjaBuild")
	if err := buildskia.NinjaBuild(path, "/usr/local/google/home/jcgregorio/projects/depot_tools", buildskia.RELEASE_BUILD, "skiaserve", 32); err != nil {
		glog.Fatalf("Failed cmake build: %s", err)
	}
	glog.Info("Starting CMakeCompileAndLink")
	cwd, err := os.Getwd()
	if err != nil {
		glog.Fatalf("Can't determine cwd: %s", err)
	}
	glog.Infof("CWD = %s", cwd)
	files := []string{
		// For fiddle only the following file needs to change to /inout/<some md5 hash>.cpp
		filepath.Join(cwd, "draw.cpp"),
		filepath.Join(path, "experimental", "fiddle", "fiddle_main.cpp"),
	}
	if err := buildskia.CMakeCompileAndLink(path, "/tmp/fiddle_main", files, []string{"-lOSMesa"}); err != nil {
		glog.Fatalf("Failed cmake build: %s", err)
	}
	files = []string{
		filepath.Join(os.Getenv("GOPATH"), "src", "go.skia.org", "infra", "webtry", "fiddle_secwrap.cpp"),
	}
	if err := buildskia.CMakeCompileAndLink(path, "fiddle_secwrap", files, []string{}); err != nil {
		glog.Fatalf("Failed cmake build: %s", err)
	}
}
