package main

import (
	"context"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/sklog"
	"google.golang.org/api/iterator"
)

// flags
var (
	outputPath = flag.String("output", "local-test-results", "Test result output path.")
	bucketName = flag.String("bucket", "", "The GCS bucket name to upload the test result to.")
)

var (
	testResultsFileName = "test_result.xml"
)

const (
	maxObjectPrefixRetries = 10
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

// generateUniqueObjectPrefix creates a unique GCS object prefix.
func generateUniqueObjectPrefix(ctx context.Context, client *storage.Client) (string, error) {
	now := time.Now().UTC()
	basePrefix := now.Format("2006-01-02/15-04-05")
	objectPrefix := basePrefix
	counter := 0

	// Create a unique GCS folder for storing the test result.
	for {
		it := client.Bucket(*bucketName).Objects(ctx, &storage.Query{Prefix: objectPrefix + "/"})
		_, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to check for existing GCS objects: %w", err)
		}

		counter++
		if counter > maxObjectPrefixRetries {
			return "", fmt.Errorf("failed to find a unique object prefix after %d tries", counter)
		}
		objectPrefix = fmt.Sprintf("%s_%d", basePrefix, counter)
	}
	return objectPrefix, nil
}

// uploadFile uploads the given file to GCS.
func uploadFile(ctx context.Context, filePath string) error {
	if *bucketName == "" {
		return nil
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("storage.NewClient: %w", err)
	}
	defer client.Close()

	objectPrefix, err := generateUniqueObjectPrefix(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to generate unique object name: %w", err)
	}
	objectName := filepath.Join(objectPrefix, testResultsFileName)

	// Open local file.
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("os.Open: %w", err)
	}
	defer f.Close()

	wc := client.Bucket(*bucketName).Object(objectName).NewWriter(ctx)
	wc.ContentType = "application/xml"

	if _, err := io.Copy(wc, f); err != nil {
		return fmt.Errorf("io.Copy: %w", err)
	}

	if err := wc.Close(); err != nil {
		return fmt.Errorf("failed to close GCS writer: %w", err)
	}

	sklog.Infof("Successfully uploaded test result to gs://%s/%s", *bucketName, objectName)
	return nil
}

// generateDummyTestResult generates a dummy test results xml.
// After adding real tests, this function must be removed.
func generateDummyTestResult() ([]byte, error) {
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
		return nil, fmt.Errorf("error marshalling XML: %w", err)
	}

	return xmlBytes, nil
}

func main() {
	flag.Parse()

	if *outputPath == "" {
		sklog.Fatal("The --output flag must be provided.")
	}

	xmlBytes, err := generateDummyTestResult()
	if err != nil {
		sklog.Fatalf("Failed to generate test result: %v", err)
	}

	if *bucketName == "" {
		if _, err := os.Stat(*outputPath); os.IsNotExist(err) {
			if err := os.MkdirAll(*outputPath, 0755); err != nil {
				sklog.Fatalf("Failed to create output directory %s: %v", *outputPath, err)
			}
		}
	}
	filePath := filepath.Join(*outputPath, testResultsFileName)
	if err := os.WriteFile(filePath, xmlBytes, 0644); err != nil {
		sklog.Fatalf("Failed to write to test result file: %v", err)
	}
	sklog.Infof("Successfully generated test result at %s", filePath)

	if err := uploadFile(context.Background(), filePath); err != nil {
		sklog.Fatalf("Failed to upload test result to GCS: %v", err)
	}
}
