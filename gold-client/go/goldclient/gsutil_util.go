package goldclient

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"

	"go.skia.org/infra/go/sklog"
)

// TODO(stephan): When we go beyond using gsutil refactor the gsUtil* functions into an
// generic upload interface and a type that implements that interface.

func gsUtilUploadJson(data interface{}, tempFileName, gcsObjPath string) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(tempFileName, jsonBytes, 0600); err != nil {
		return err
	}

	// Upload the written file.
	return gsUtilCopyToGCS(tempFileName, gcsObjPath)
}

// gsUtilCopyToGCS from local file system to GCS.
func gsUtilCopyToGCS(localSrc, gcsTargetPath string) error {
	return gsutilCopy(localSrc, fmt.Sprintf("gs://%s", gcsTargetPath))
}

func gsUtilCpFromGCS(gcsSrcPath, targetFilePath string) error {
	return gsutilCopy(fmt.Sprintf("gs://%s", gcsSrcPath), targetFilePath)
}

func gsutilCopy(src, tgt string) error {
	runCmd := exec.Command("gsutil", "cp", src, tgt)
	outBytes, err := runCmd.CombinedOutput()
	if err != nil {
		return sklog.FmtErrorf("Error running gsutil. Got output \n%s\n and error: %s", outBytes, err)
	}
	return nil
}
