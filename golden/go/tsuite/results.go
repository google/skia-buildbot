package tsuite

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"go.skia.org/infra/go/fileutil"
)

// Format of how Firebase Test Lab returns the device information.
type FirebaseDevice struct {
	Brand        string   `json:"brand"`
	Form         string   `json:"form"`
	ID           string   `json:"id"`
	Manufacturer string   `json:"manufacturer"`
	Name         string   `json:"name"`
	VersionIDs   []string `json:"supportedVersionIds"`
}

type DeviceVersions struct {
	Device   *FirebaseDevice
	Versions []string
}

type TestRunMeta struct {
	ID             string            `json:"id"`
	TS             int64             `json:"timeStamp"`
	Devices        []*DeviceVersions `json:"devices"`
	IgnoredDevices []*DeviceVersions `json:"ignoredDevices"`
	ExitCode       int               `json:"exitCode"`
}

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

func LoadSuiteResults(r io.Reader) (*SuiteResult, error) {
	ret := &SuiteResult{}
	if err := json.NewDecoder(r).Decode(ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func LoadTestRunMeta(r io.Reader) (*TestRunMeta, error) {
	ret := &TestRunMeta{}
	if err := json.NewDecoder(r).Decode(ret); err != nil {
		return nil, err
	}
	return ret, nil
}
