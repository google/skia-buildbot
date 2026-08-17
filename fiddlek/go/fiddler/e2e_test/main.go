package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"go.skia.org/infra/fiddlek/go/types"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	imageName     = "gcr.io/skia-public/fiddler-base:latest"
	containerName = "fiddler_e2e_test_go"
	fiddlerPort   = "8080"
	logFile       = "/tmp/fiddler.log"
)

// Test cases.
var (
	//go:embed test_cases/success.cpp
	testCaseSuccess string

	//go:embed test_cases/fail_symlink.cpp
	testCaseFailSymlink string

	//go:embed test_cases/fail_readlink.cpp
	testCaseFailReadlink string

	//go:embed test_cases/fail_socket.cpp
	testCaseFailSocket string

	//go:embed test_cases/fail_execve.cpp
	testCaseFailExecve string

	//go:embed test_cases/fail_link.cpp
	testCaseFailLink string

	//go:embed test_cases/fail_rename.cpp
	testCaseFailRename string

	//go:embed test_cases/fail_mknod.cpp
	testCaseFailMknod string

	//go:embed test_cases/fail_proc_self.cpp
	testCaseFailProcSelf string
)

// Files used to mock the Skia repo.
var (
	//go:embed skia_mock/VERSION
	mockVersion string

	//go:embed skia_mock/main.cpp
	mockMainCpp string

	//go:embed skia_mock/draw.cpp
	mockDrawCpp string

	//go:embed skia_mock/build.ninja
	mockBuildNinja string
)

func runFiddlerTest(ctx context.Context, testName string, code string, expectSuccess bool) error {
	sklog.Infof("--- Running test: %s ---", testName)

	// Ensure a fresh build by deleting the old binary inside the container.
	_, err := exec.RunCwd(ctx, ".", "docker", "exec", "-u", "skia", containerName, "rm", "-f", "/tmp/skia_mock/out/Static/fiddle", logFile)
	if err != nil {
		return skerr.Wrapf(err, "failed to clean up old fiddle binary")
	}

	// Start fiddler in the background.
	fiddlerCmd := fmt.Sprintf("export PATH=/usr/local/bin:$PATH && /usr/local/bin/fiddler --local --fiddle_root=/tmp --checkout=/tmp/skia_mock --port=:%s > %s 2>&1", fiddlerPort, logFile)
	args := append([]string{"docker", "exec", "-d", "-u", "skia", containerName}, "bash", "-c", fiddlerCmd)
	_, err = exec.RunCwd(ctx, ".", args...)
	if err != nil {
		return skerr.Wrapf(err, "failed to start fiddler")
	}

	// Wait for fiddler to start.
	c := httputils.NewTimeoutClient()
	url := fmt.Sprintf("http://localhost:%s", fiddlerPort)
	started := false
	for i := 0; i < 10; i++ {
		resp, err := c.Get(url + "/")
		if err == nil {
			resp.Body.Close()
			started = true
			break
		}
		time.Sleep(time.Second)
	}
	if !started {
		return skerr.Fmt("fiddler failed to start")
	}

	// Send request.
	requestData := types.FiddleContext{
		Code: code,
		Options: types.Options{
			Width:    128,
			Height:   128,
			TextOnly: true,
		},
		Fast: false,
	}
	requestBody, err := json.Marshal(requestData)
	if err != nil {
		return skerr.Wrapf(err, "failed to marshal request")
	}

	// Read the response.
	resp, err := c.Post(url+"/run", "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return skerr.Wrapf(err, "failed to send request")
	}
	defer resp.Body.Close()
	var res types.Result
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return skerr.Wrapf(err, "failed to decode response")
	}

	// Check results.
	if res.Compile.Errors != "" {
		return skerr.Fmt("expected successful compilation, but got errors: %s", res.Compile.Errors)
	}
	if expectSuccess && res.Execute.Errors != "" {
		return skerr.Fmt("expected successful execution, but got errors: %s", res.Execute.Errors)
	} else if !expectSuccess {
		if res.Execute.Errors == "" {
			resJSON, _ := json.MarshalIndent(res, "", "  ")
			// Read the logs from inside the container to see what happened.
			var logStdout bytes.Buffer
			_ = exec.Run(ctx, &exec.Command{
				Name:   "docker",
				Args:   []string{"exec", containerName, "cat", logFile},
				Stdout: &logStdout,
			})
			return skerr.Fmt("expected failure (compile or execute error), but both were empty in test: %s\nResponse: %s\nFiddler Logs:\n%s", testName, string(resJSON), logStdout.String())
		}
		// If the code runs and prints BYPASS SUCCESS, then the bypass worked (which is what we are testing).
		if strings.Contains(res.Execute.Output.Text, "BYPASS SUCCESS") {
			return skerr.Fmt("security bypass succeeded! Vulnerability found in test: %s", testName)
		}
	}
	resJSON, _ := json.MarshalIndent(res, "", "  ")
	// Read the logs from inside the container to see what happened.
	var logStdout bytes.Buffer
	_ = exec.Run(ctx, &exec.Command{
		Name:   "docker",
		Args:   []string{"exec", containerName, "cat", logFile},
		Stdout: &logStdout,
	})
	fmt.Fprintf(os.Stderr, "%s\nResponse: %s\nFiddler Logs:\n%s\n", testName, string(resJSON), logStdout.String())
	sklog.Infof("PASSED: %s", testName)
	return nil
}

func writeFileInContainer(ctx context.Context, user, path, content string) error {
	cmd := &exec.Command{
		Name:   "docker",
		Args:   []string{"exec", "-i", "-u", user, containerName, "tee", path},
		Stdin:  strings.NewReader(content),
		Stdout: nil,
	}
	_, err := exec.RunCommand(ctx, cmd)
	return err
}

func main() {
	ctx := context.Background()

	// Ensure the image exists.
	_, err := exec.RunCwd(ctx, ".", "docker", "image", "inspect", imageName)
	if err != nil {
		sklog.Fatalf("Container image %s must be loaded before running this test.", imageName)
	}

	// Run the container.
	_, _ = exec.RunCwd(ctx, ".", "docker", "rm", "-f", containerName)
	_, err = exec.RunCwd(ctx, ".", "docker", "run", "--name", containerName, "-d", "-p", fiddlerPort+":"+fiddlerPort, "--entrypoint", "/bin/bash", imageName, "-c", "sleep infinity")
	if err != nil {
		sklog.Fatalf("failed to run container: %v", err)
	}
	defer func() {
		_, _ = exec.RunCwd(ctx, ".", "docker", "rm", "-f", containerName)
	}()

	// Prepare mock Skia environment.
	setupCommands := [][]string{
		{"mkdir", "-p", "/tmp/skia_mock/tools/fiddle", "/tmp/skia_mock/out/Static"},
		{"chown", "-R", "skia:skia", "/tmp/skia_mock"},
	}
	for _, cmdArgs := range setupCommands {
		args := append([]string{"docker", "exec", "-u", "root", containerName}, cmdArgs...)
		_, err = exec.RunCwd(ctx, ".", args...)
		if err != nil {
			sklog.Fatalf("failed to setup environment: %v", err)
		}
	}
	filesToCreate := map[string]string{
		"/tmp/skia_mock/VERSION":                mockVersion,
		"/tmp/skia_mock/tools/fiddle/main.cpp":  mockMainCpp,
		"/tmp/skia_mock/tools/fiddle/draw.cpp":  mockDrawCpp,
		"/tmp/skia_mock/out/Static/build.ninja": mockBuildNinja,
	}
	for path, content := range filesToCreate {
		if err := writeFileInContainer(ctx, "skia", path, content); err != nil {
			sklog.Fatalf("failed to create file %s: %v", path, err)
		}
	}

	// Run test cases.
	tests := []struct {
		name          string
		code          string
		expectSuccess bool
	}{
		{"Success", testCaseSuccess, true},
		{"Fail Symlink", testCaseFailSymlink, false},
		{"Fail Readlink", testCaseFailReadlink, false},
		{"Fail Socket", testCaseFailSocket, false},
		{"Fail Execve", testCaseFailExecve, false},
		{"Fail Link", testCaseFailLink, false},
		{"Fail Rename", testCaseFailRename, false},
		{"Fail Mknod", testCaseFailMknod, false},
		{"Fail /proc/self", testCaseFailProcSelf, false},
	}

	for _, tc := range tests {
		if err := runFiddlerTest(ctx, tc.name, tc.code, tc.expectSuccess); err != nil {
			sklog.Fatal(err)
		}
	}
}
