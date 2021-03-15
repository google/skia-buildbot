package isolate

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

// Upload the files specified by the given isolateFile to the isolate server.
// Returns the CAS digest.
func Upload(ctx context.Context, casInstance, baseDir, isolateFile string) (string, error) {
	// Setup.
	tmpDir, err := ioutil.TempDir("", "isolate")
	if err != nil {
		return "", skerr.Wrapf(err, "failed to create temporary dir")
	}
	defer util.RemoveAll(tmpDir)

	// Isolate the tasks.
	jsonOutput := filepath.Join(tmpDir, "isolated.json")
	cmd := []string{
		"isolate", "archive", "--verbose",
		"--cas-instance", casInstance,
		"--dump-json", jsonOutput,
		"--isolate", isolateFile,
	}

	baseDirAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	if _, err := exec.RunCwd(ctx, baseDirAbs, cmd...); err != nil {
		return "", skerr.Wrap(err)
	}

	// Read the JSON output file and return the hash.
	b, err := ioutil.ReadFile(jsonOutput)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	var hashes map[string]string
	if err := json.Unmarshal(b, &hashes); err != nil {
		return "", skerr.Wrap(err)
	}
	// We only provided one task, so there should only be one key in the map.
	if len(hashes) != 1 {
		return "", skerr.Fmt("Expected 1 hash but got %d; output: %s", len(hashes), string(b))
	}
	for _, hash := range hashes {
		return hash, nil
	}
	return "", skerr.Fmt("Don't know how to read isolated output")
}
