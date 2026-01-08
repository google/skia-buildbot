package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

// flags
var (
	outputPath = flag.String("output", "test_result.xml", "File path for dropping the generated test result file.")
)

// TestSuites is the root element for xUnit XML reports.
type TestSuites struct {
	XMLName   xml.Name    `xml:"testsuites"`
	Name      string      `xml:"name,attr"`
	TestSuite []TestSuite `xml:"testsuite"`
}

// TestSuite represents a single suite of tests.
type TestSuite struct {
	XMLName   xml.Name   `xml:"testsuite"`
	Name      string     `xml:"name,attr"`
	Tests     int        `xml:"tests,attr"`
	Failures  int        `xml:"failures,attr"`
	Errors    int        `xml:"errors,attr"`
	Skipped   int        `xml:"skipped,attr"`
	Timestamp string     `xml:"timestamp,attr"`
	Time      string     `xml:"time,attr"`
	TestCases []TestCase `xml:"testcase"`
}

// TestCase represents a single test case.
type TestCase struct {
	XMLName   xml.Name `xml:"testcase"`
	Name      string   `xml:"name,attr"`
	ClassName string   `xml:"classname,attr"`
	Time      string   `xml:"time,attr"`
}

// generateDummyTestResultFile generates a dummy test results xml file.
// After adding real tests, this function must be removed.
func generateDummyTestResultFile(outputPath string) error {
	suites := TestSuites{
		Name: "results",
		TestSuite: []TestSuite{
			{
				Name:      "dummy suite",
				Tests:     1,
				Failures:  0,
				Errors:    0,
				Skipped:   0,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Time:      "0.1",
				TestCases: []TestCase{
					{Name: "dummy test", ClassName: "dummy.class", Time: "0.1"},
				},
			},
		},
	}

	xmlBytes, err := xml.MarshalIndent(suites, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling XML: %w", err)
	}

	if err := os.WriteFile(outputPath, xmlBytes, 0644); err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}
	return nil
}

func main() {
	flag.Parse()

	// TODO(faridzad): Remove this after adding e2e tests.
	if err := generateDummyTestResultFile(*outputPath); err != nil {
		log.Fatalf("Failed to generate test result file: %v", err)
	}

	log.Printf("Successfully generated dummy test result at %s", *outputPath)
}
