package goldclient

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"

	"go.skia.org/infra/go/skerr"
)

// gsutilAvailable returns true if the 'gsutil' command could be found on the PATH
func gsutilAvailable() bool {
	_, err := exec.LookPath("gsutil")
	return err == nil
}

// gsUtilUploadJson serializes the given data to JSON and writes the result to the given
// tempFileName, then it copies the file to the given path in GCS. gcsObjPath is assumed
// to have the form: <bucket_name>/path/to/object
func gsUtilUploadJson(data interface{}, tempFileName, gcsObjPath string) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(tempFileName, jsonBytes, 0644); err != nil {
		return err
	}

	// Upload the written file.
	return gsutilCopy(tempFileName, prefixGCS(gcsObjPath))
}

// prefixGCS adds the "gs://" prefix to the given GCS path.
func prefixGCS(gcsPath string) string {
	return fmt.Sprintf("gs://%s", gcsPath)
}

// gsutilCopy shells out to gsutil to copy the given src to the given target. A path
// starting with "gs://" is assumed to be in GCS.
func gsutilCopy(src, dst string) error {
	runCmd := exec.Command("gsutil", "cp", src, dst)
	outBytes, err := runCmd.CombinedOutput()
	if err != nil {
		return skerr.Fmt("Error running gsutil. Got output \n%s\n and error: %s", outBytes, err)
	}
	return nil
}
