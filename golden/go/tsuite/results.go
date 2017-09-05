package tsuite

import (
	"encoding/json"
	"fmt"
	"os"

	"go.skia.org/infra/go/fileutil"
)

type ResultStatus string

const (
	OK            ResultStatus = "OK"
	ERR           ResultStatus = "FAIL"
	SAVE_FAILED   ResultStatus = "DISK"
	ENCODE_FAILED ResultStatus = "ENCODE"
)

type TestResult struct {
	Name     string       `json:"name"`
	Status   ResultStatus `json:"status"`
	ErrorMsg string       `json:"errorMsg"`
}

type SuiteResult struct {
	Results []*TestResult `json:"results"`
}

func (s *SuiteResult) Add(testName string, status ResultStatus, errMsg string) {
	s.Results = append(s.Results, &TestResult{
		Name:     testName,
		Status:   status,
		ErrorMsg: errMsg,
	})
}

func (s *SuiteResult) WriteToFile(fName string) error {
	if err := fileutil.EnsureDirPathExists(fName); err != nil {
		return fmt.Errorf("Unable to open directory %s: %s", fName, err)
	}

	outFile, err := os.Create(fName)
	if err != nil {
		return fmt.Errorf("Unable to create files %s: %s", fName, err)
	}
	defer func() { _ = outFile.Close() }()

	if err := json.NewEncoder(outFile).Encode(s); err != nil {
		return fmt.Errorf("Unable to encode results: %s", err)
	}
	return nil
}
