package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/secret/mocks"
)

const (
	oldProject       = "old-project"
	oldSecretName    = "old-secret-name"
	oldSecretSubPath = "old-secret-subpath"
	testProject      = "test-project"
	testSecretName   = "test-secret-name"
	testSecretValue  = "test-secret-value"
	newSecretValue   = "new-secret-value"
)

var (
	secretLocationRegex = regexp.MustCompile(`Wrote secret to (.+)\n`)
)

func stdoutWatcher(t *testing.T) (io.Writer, <-chan string) {
	// Read from stdout, watch for the secret file location.
	stdoutReader, stdout := io.Pipe()
	secretFileCh := make(chan string)
	go func() {
		for {
			stdoutBuf := make([]byte, 256)
			n, err := stdoutReader.Read(stdoutBuf)
			if err == io.EOF {
				return
			}
			require.NoError(t, err)
			matches := secretLocationRegex.FindStringSubmatch(string(stdoutBuf[:n]))
			if len(matches) == 2 {
				secretFileCh <- matches[1]
			}
		}
	}()
	t.Cleanup(func() {
		require.NoError(t, stdout.Close())
	})
	return stdout, secretFileCh
}

func setup(t *testing.T) (context.Context, *secretsApp, *mocks.Client, <-chan string, chan<- bool) {

	mockClient := mocks.NewClient(t)
	stdin, stdinWriter := io.Pipe()
	stdout, secretFileCh := stdoutWatcher(t)
	enterToContinueCh := make(chan bool)
	go func() {
		<-enterToContinueCh
		_, err := stdinWriter.Write([]byte("\n"))
		require.NoError(t, err)
	}()
	app := &secretsApp{
		secretClient: mockClient,
		stdin:        stdin,
		stdout:       stdout,
	}
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_Mount",
		"Test_FakeExe_Umount",
	)
	return ctx, app, mockClient, secretFileCh, enterToContinueCh
}

func TestSecretsApp_Create(t *testing.T) {
	ctx, app, mockClient, secretFileCh, enterToContinueCh := setup(t)
	mockClient.On("Create", ctx, testProject, testSecretName).Return(nil)
	mockClient.On("Update", ctx, testProject, testSecretName, testSecretValue).Return("1", nil)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		require.NoError(t, app.cmdCreate(ctx, testProject, testSecretName))
	}()
	secretFilePath := <-secretFileCh
	secretFileContents, err := ioutil.ReadFile(secretFilePath)
	require.NoError(t, err)
	require.Equal(t, "", string(secretFileContents))
	require.NoError(t, ioutil.WriteFile(secretFilePath, []byte(testSecretValue), os.ModePerm))
	enterToContinueCh <- true
	wg.Wait()
}

func TestSecretsApp_Update(t *testing.T) {
	ctx, app, mockClient, secretFileCh, enterToContinueCh := setup(t)
	mockClient.On("Get", ctx, testProject, testSecretName, secret.VersionLatest).Return(testSecretValue, nil)
	mockClient.On("Update", ctx, testProject, testSecretName, newSecretValue).Return("2", nil)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		require.NoError(t, app.cmdUpdate(ctx, testProject, testSecretName))
	}()
	secretFilePath := <-secretFileCh
	secretFileContents, err := ioutil.ReadFile(secretFilePath)
	require.NoError(t, err)
	require.Equal(t, testSecretValue, string(secretFileContents))
	require.NoError(t, ioutil.WriteFile(secretFilePath, []byte(newSecretValue), os.ModePerm))
	enterToContinueCh <- true
	wg.Wait()
}

func TestSecretsApp_Migrate(t *testing.T) {

	mockClient := mocks.NewClient(t)
	app := &secretsApp{
		secretClient: mockClient,
		stdin:        os.Stdin,
		stdout:       os.Stdout,
	}
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_KubectlGetSecret",
	)
	mockClient.On("Create", ctx, testProject, testSecretName).Return(nil)
	mockClient.On("Update", ctx, testProject, testSecretName, testSecretValue).Return("1", nil)
	require.NoError(t, app.cmdMigrate(ctx, oldProject, oldSecretName, oldSecretSubPath, testProject, testSecretName))
}

func Test_FakeExe_Mount(t *testing.T) {
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}
	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"sudo", "mount", "-t", "tmpfs", "-o", "size=10m", "tmpfs"}, args[:len(args)-1])
}

func Test_FakeExe_Umount(t *testing.T) {
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}
	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"sudo", "umount"}, args[:len(args)-1])
}

func Test_FakeExe_KubectlGetSecret(t *testing.T) {
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}
	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{oldProject, "kubectl", "get", "secret", oldSecretName, "-o", "jsonpath={.data}"}, args[1:])

	require.NoError(t, json.NewEncoder(os.Stdout).Encode(map[string]string{
		oldSecretSubPath: base64.StdEncoding.EncodeToString([]byte(testSecretValue)),
	}))
	os.Exit(0)
}
