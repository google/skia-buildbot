package repo_manager

/*
   This file contains canned pre-upload steps for RepoManagers to use.
*/

import (
	"fmt"
	"path"

	"go.skia.org/infra/go/exec"
)

// PreUploadStep is a function to be run after the roll is performed but before
// a CL is uploaded. The string parameter is the directory of the parent repo.
type PreUploadStep func(string) error

// Return the PreUploadStep with the given name.
func GetPreUploadStep(s string) (PreUploadStep, error) {
	rv, ok := map[string]PreUploadStep{
		"TrainInfra": TrainInfra,
	}[s]
	if !ok {
		return nil, fmt.Errorf("No such pre-upload step: %s", s)
	}
	return rv, nil
}

// Return the PreUploadSteps with the given names.
func GetPreUploadSteps(steps []string) ([]PreUploadStep, error) {
	rv := make([]PreUploadStep, 0, len(steps))
	for _, s := range steps {
		step, err := GetPreUploadStep(s)
		if err != nil {
			return nil, err
		}
		rv = append(rv, step)
	}
	return rv, nil
}

// Train the infra expectations.
func TrainInfra(parentDir string) error {
	if _, err := exec.RunCwd(path.Join(parentDir, "infra", "bots"), "make", "train"); err != nil {
		return err
	}
	return nil
}
