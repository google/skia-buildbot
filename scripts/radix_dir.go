package main

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/fileutil"
)

func main() {
	common.Init()

	fileInfos, err := ioutil.ReadDir(".")
	if err != nil {
		glog.Fatalf("Unable to read directory.")
	}

	// Get the directory
	for _, info := range fileInfos {
		if info.IsDir() {
			continue
		}

		fileName := info.Name()
		outFileName := fileutil.TwoLevelRadixPath(fileName)

		if fileName == outFileName {
			glog.Infof("Excluding %s -> %s", fileName, outFileName)
			continue
		}

		if !fileutil.FileExists(outFileName) {
			// Create the path if it doesn't exist.
			targetDir, _ := filepath.Split(outFileName)
			if err = os.MkdirAll(targetDir, 0700); err != nil {
				glog.Errorf("Unable to run create path: %s", targetDir)
			}

			if err := os.Rename(fileName, outFileName); err != nil {
				glog.Errorf("Unable to run mv %s %s", fileName, outFileName)
			}
		}

	}
}
