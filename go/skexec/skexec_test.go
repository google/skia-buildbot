package skexec

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.skia.org/infra/go/testutils"

	expect "github.com/stretchr/testify/assert"
	assert "github.com/stretchr/testify/require"
)

func TestParseCommand(t *testing.T) {
	testutils.SmallTest(t)
	test := func(input string, expected Command) {
		expect.Equal(t, expected, ParseCommand(input))
	}
	test("", Command{Name: "", Args: []string{}})
	test("foo", Command{Name: "foo", Args: []string{}})
	test("foo bar", Command{Name: "foo", Args: []string{"bar"}})
	test("foo_bar baz", Command{Name: "foo_bar", Args: []string{"baz"}})
	test("foo-bar baz", Command{Name: "foo-bar", Args: []string{"baz"}})
	test("foo --bar --baz", Command{Name: "foo", Args: []string{"--bar", "--baz"}})
	// Doesn't work.
	//test("foo 'bar baz'", Command{Name: "foo", Args: []string{"bar baz"}})
}

func TempDir(t *testing.T, name string) string {
	dir, err := ioutil.TempDir("", name)
	assert.NoError(t, err)
	return dir
}

func TestBasic(t *testing.T) {
	testutils.SmallTest(t)
	dir := TempDir(t, "skexec_test_")
	defer testutils.RemoveAll(t, dir)
	file := filepath.Join(dir, "ran")
	assert.NoError(t, NewExec().Run(&Command{
		Name: "touch",
		Args: []string{file},
	}))
	_, err := os.Stat(file)
	expect.NoError(t, err)
}

func WriteScript(t *testing.T, path, script string) {
	assert.NoError(t, ioutil.WriteFile(path, []byte(script), 0777))
}

const SimpleScript = `#!/bin/bash
touch "${EXEC_TEST_FILE}"
`

func TestEnv(t *testing.T) {
	testutils.SmallTest(t)
	dir := TempDir(t, "skexec_test_")
	defer testutils.RemoveAll(t, dir)
	script := filepath.Join(dir, "simple_script.sh")
	WriteScript(t, script, SimpleScript)
	file := filepath.Join(dir, "ran")
	assert.NoError(t, NewExec().Run(&Command{
		Name: script,
		Env:  []string{fmt.Sprintf("EXEC_TEST_FILE=%s", file)},
	}))
	_, err := os.Stat(file)
	expect.NoError(t, err)
}

const PathScript = `#!/bin/bash
echo "${PATH}" > "${EXEC_TEST_FILE}"
`

func TestInheritPath(t *testing.T) {
	testutils.SmallTest(t)
	dir := TempDir(t, "skexec_test_")
	defer testutils.RemoveAll(t, dir)
	script := filepath.Join(dir, "path_script.sh")
	WriteScript(t, script, PathScript)
	file := filepath.Join(dir, "ran")
	assert.NoError(t, NewExec().Run(&Command{
		Name:        script,
		Env:         []string{fmt.Sprintf("EXEC_TEST_FILE=%s", file)},
		InheritPath: true,
	}))
	contents, err := ioutil.ReadFile(file)
	assert.NoError(t, err)
	expect.Equal(t, os.Getenv("PATH"), strings.TrimSpace(string(contents)))
}

// Add x before variable to ensure no blank lines.
const EnvScript = `#!/bin/bash
echo "x${PATH}" > "${EXEC_TEST_FILE}"
echo "x${USER}" >> "${EXEC_TEST_FILE}"
echo "x${PWD}" >> "${EXEC_TEST_FILE}"
echo "${HOME}" >> "${EXEC_TEST_FILE}"
echo "x${GOPATH}" >> "${EXEC_TEST_FILE}"
`

func TestInheritEnv(t *testing.T) {
	testutils.SmallTest(t)
	dir := TempDir(t, "skexec_test_")
	defer testutils.RemoveAll(t, dir)
	script := filepath.Join(dir, "path_script.sh")
	WriteScript(t, script, EnvScript)
	file := filepath.Join(dir, "ran")
	assert.NoError(t, NewExec().Run(&Command{
		Name: script,
		Env: []string{
			fmt.Sprintf("EXEC_TEST_FILE=%s", file),
			fmt.Sprintf("HOME=%s", dir),
		},
		InheritPath: false,
		InheritEnv:  true,
	}))
	contents, err := ioutil.ReadFile(file)
	assert.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(contents)), "\n")
	assert.Len(t, lines, 5)
	expect.Equal(t, "x"+os.Getenv("PATH"), lines[0])
	expect.Equal(t, "x"+os.Getenv("USER"), lines[1])
	expect.Equal(t, "x"+os.Getenv("PWD"), lines[2])
	expect.Equal(t, dir, lines[3])
	expect.Equal(t, "x"+os.Getenv("GOPATH"), lines[4])
}

const HelloScript = `#!/bin/bash
echo "Hello World!" > output.txt
`

func TestDir(t *testing.T) {
	testutils.SmallTest(t)
	dir1 := TempDir(t, "exec_test1")
	defer testutils.RemoveAll(t, dir1)
	script := filepath.Join(dir1, "hello_script.sh")
	WriteScript(t, script, HelloScript)

	dir2 := TempDir(t, "exec_test2")
	defer testutils.RemoveAll(t, dir2)
	assert.NoError(t, NewExec().Run(&Command{
		Name: script,
		Dir:  dir2,
	}))
	_, err := os.Stat(filepath.Join(dir2, "output.txt"))
	expect.NoError(t, err)
}

func TestSimpleIO(t *testing.T) {
	testutils.SmallTest(t)
	inputString := "foo\nbar\nbaz\n"
	output := bytes.Buffer{}
	assert.NoError(t, NewExec().Run(&Command{
		Name:   "grep",
		Args:   []string{"-e", "^ba"},
		Stdin:  bytes.NewReader([]byte(inputString)),
		Stdout: &output,
	}))
	expect.Equal(t, "bar\nbaz\n", output.String())
}

func TestSimpleIOReturnOutput(t *testing.T) {
	testutils.SmallTest(t)
	inputString := "foo\nbar\nbaz\n"
	out, err := NewExec().GetOutput(&Command{
		Name:  "grep",
		Args:  []string{"-e", "^ba"},
		Stdin: bytes.NewReader([]byte(inputString)),
	})
	assert.NoError(t, err)
	expect.Equal(t, "bar\nbaz\n", out)
}

func TestStderr(t *testing.T) {
	testutils.SmallTest(t)
	dir := TempDir(t, "skexec_test_")
	defer testutils.RemoveAll(t, dir)
	output := bytes.Buffer{}
	err := NewExec().Run(&Command{
		Name: "cp",
		Args: []string{filepath.Join(dir, "doesnt_exist"),
			filepath.Join(dir, "dest")},
		Stderr: &output,
	})
	expect.Error(t, err)
	expect.Contains(t, err.Error(), "exit status 1")
	expect.Contains(t, output.String(), "No such file or directory")
}

func TestStderrGetOutput(t *testing.T) {
	testutils.SmallTest(t)
	dir := TempDir(t, "skexec_test_")
	defer testutils.RemoveAll(t, dir)
	out, err := NewExec().GetOutput(&Command{
		Name: "cp",
		Args: []string{filepath.Join(dir, "doesnt_exist"),
			filepath.Join(dir, "dest")},
	})
	expect.Error(t, err)
	expect.Contains(t, err.Error(), "exit status 1")
	expect.Contains(t, out, "No such file or directory")
}

const StderrStdoutScript = `#!/bin/bash
echo "roses"
>&2 echo "red"
echo "violets"
>&2 echo "blue"
`

func TestCombinedOutput(t *testing.T) {
	testutils.SmallTest(t)
	dir := TempDir(t, "skexec_test_")
	defer testutils.RemoveAll(t, dir)
	script := filepath.Join(dir, "combined_output_script.sh")
	WriteScript(t, script, StderrStdoutScript)
	combined := bytes.Buffer{}
	assert.NoError(t, NewExec().Run(&Command{
		Name:   script,
		Stdout: &combined,
		Stderr: &combined,
	}))
	expect.Equal(t, "roses\nred\nviolets\nblue\n", combined.String())
}

func TestCombinedOutputWithLogging(t *testing.T) {
	testutils.SmallTest(t)
	dir := TempDir(t, "skexec_test_")
	defer testutils.RemoveAll(t, dir)
	script := filepath.Join(dir, "combined_output_script.sh")
	WriteScript(t, script, StderrStdoutScript)
	combined := bytes.Buffer{}
	assert.NoError(t, NewExec().Run(&Command{
		Name:      script,
		Stdout:    &combined,
		LogStdout: true,
		Stderr:    &combined,
		LogStderr: true,
	}))
	combinedStr := combined.String()
	// Due to adding logging, the order of output is non-deterministic.
	expect.Regexp(t, "(?s)roses\n.*violets\n", combinedStr)
	expect.Regexp(t, "(?s)red\n.*blue\n", combinedStr)
}

func TestSeparateOutput(t *testing.T) {
	testutils.SmallTest(t)
	dir := TempDir(t, "skexec_test_")
	defer testutils.RemoveAll(t, dir)
	script := filepath.Join(dir, "combined_output_script.sh")
	WriteScript(t, script, StderrStdoutScript)
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	assert.NoError(t, NewExec().Run(&Command{
		Name:   script,
		Stdout: &stdout,
		Stderr: &stderr,
	}))
	expect.Equal(t, "roses\nviolets\n", stdout.String())
	expect.Equal(t, "red\nblue\n", stderr.String())
}

func TestSeparateOutputWithLogging(t *testing.T) {
	testutils.SmallTest(t)
	dir := TempDir(t, "skexec_test_")
	defer testutils.RemoveAll(t, dir)
	script := filepath.Join(dir, "combined_output_script.sh")
	WriteScript(t, script, StderrStdoutScript)
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	assert.NoError(t, NewExec().Run(&Command{
		Name:      script,
		Stdout:    &stdout,
		LogStdout: true,
		Stderr:    &stderr,
		LogStderr: true,
	}))
	expect.Equal(t, "roses\nviolets\n", stdout.String())
	expect.Equal(t, "red\nblue\n", stderr.String())
}

func TestGetOutput(t *testing.T) {
	testutils.SmallTest(t)
	dir := TempDir(t, "skexec_test_")
	defer testutils.RemoveAll(t, dir)
	script := filepath.Join(dir, "combined_output_script.sh")
	WriteScript(t, script, StderrStdoutScript)
	out, err := NewExec().GetOutput(&Command{
		Name: script,
	})
	assert.NoError(t, err)
	expect.Equal(t, "roses\nred\nviolets\nblue\n", out)
}

func TestGetOutputWithLogging(t *testing.T) {
	testutils.SmallTest(t)
	dir := TempDir(t, "skexec_test_")
	defer testutils.RemoveAll(t, dir)
	script := filepath.Join(dir, "combined_output_script.sh")
	WriteScript(t, script, StderrStdoutScript)
	out, err := NewExec().GetOutput(&Command{
		Name:      script,
		LogStdout: true,
		LogStderr: true,
	})
	assert.NoError(t, err)
	// Due to adding logging, the order of output is non-deterministic.
	expect.Regexp(t, "(?s)roses\n.*violets\n", out)
	expect.Regexp(t, "(?s)red\n.*blue\n", out)
}

func TestGetOutputCaptureStdout(t *testing.T) {
	testutils.SmallTest(t)
	dir := TempDir(t, "skexec_test_")
	defer testutils.RemoveAll(t, dir)
	script := filepath.Join(dir, "combined_output_script.sh")
	WriteScript(t, script, StderrStdoutScript)
	stdout := bytes.Buffer{}
	out, err := NewExec().GetOutput(&Command{
		Name:   script,
		Stdout: &stdout,
	})
	assert.NoError(t, err)
	expect.Equal(t, "roses\nviolets\n", stdout.String())
	expect.Equal(t, "red\nblue\n", out)
}

func TestGetOutputCaptureStdoutWithLogging(t *testing.T) {
	testutils.SmallTest(t)
	dir := TempDir(t, "skexec_test_")
	defer testutils.RemoveAll(t, dir)
	script := filepath.Join(dir, "combined_output_script.sh")
	WriteScript(t, script, StderrStdoutScript)
	stdout := bytes.Buffer{}
	out, err := NewExec().GetOutput(&Command{
		Name:      script,
		Stdout:    &stdout,
		LogStdout: true,
		LogStderr: true,
	})
	assert.NoError(t, err)
	expect.Equal(t, "roses\nviolets\n", stdout.String())
	expect.Equal(t, "red\nblue\n", out)
}

func TestGetOutputCaptureStderr(t *testing.T) {
	testutils.SmallTest(t)
	dir := TempDir(t, "skexec_test_")
	defer testutils.RemoveAll(t, dir)
	script := filepath.Join(dir, "combined_output_script.sh")
	WriteScript(t, script, StderrStdoutScript)
	stderr := bytes.Buffer{}
	out, err := NewExec().GetOutput(&Command{
		Name:   script,
		Stderr: &stderr,
	})
	assert.NoError(t, err)
	expect.Equal(t, "roses\nviolets\n", out)
	expect.Equal(t, "red\nblue\n", stderr.String())
}

func TestGetOutputCaptureStderrWithLogging(t *testing.T) {
	testutils.SmallTest(t)
	dir := TempDir(t, "skexec_test_")
	defer testutils.RemoveAll(t, dir)
	script := filepath.Join(dir, "combined_output_script.sh")
	WriteScript(t, script, StderrStdoutScript)
	stderr := bytes.Buffer{}
	out, err := NewExec().GetOutput(&Command{
		Name:      script,
		LogStdout: true,
		Stderr:    &stderr,
		LogStderr: true,
	})
	assert.NoError(t, err)
	expect.Equal(t, "roses\nviolets\n", out)
	expect.Equal(t, "red\nblue\n", stderr.String())
}

// Previously there was a bug due to code like:
// var outputFile *os.File
// if outputToFile {
// 	outputFile = ...
// }
// NewExec().Run(&Command{... Stdout: outputFile})
// See http://devs.cloudimmunity.com/gotchas-and-common-mistakes-in-go-golang/index.html#nil_in_nil_in_vals
func TestNilIO(t *testing.T) {
	testutils.SmallTest(t)
	inputString := "foo\nbar\nbaz\n"
	assert.NoError(t, NewExec().Run(&Command{
		Name:   "grep",
		Args:   []string{"-e", "^ba"},
		Stdin:  bytes.NewReader([]byte(inputString)),
		Stdout: (*os.File)(nil),
	}))
}

func TestNilIOWithLogging(t *testing.T) {
	testutils.SmallTest(t)
	inputString := "foo\nbar\nbaz\n"
	assert.NoError(t, NewExec().Run(&Command{
		Name:      "grep",
		Args:      []string{"-e", "^ba"},
		Stdin:     bytes.NewReader([]byte(inputString)),
		Stdout:    (*os.File)(nil),
		LogStdout: true,
	}))
}

func TestNilIOGetOutput(t *testing.T) {
	testutils.SmallTest(t)
	inputString := "foo\nbar\nbaz\n"
	out, err := NewExec().GetOutput(&Command{
		Name:   "grep",
		Args:   []string{"-e", "^ba"},
		Stdin:  bytes.NewReader([]byte(inputString)),
		Stdout: (*os.File)(nil),
	})
	assert.NoError(t, err)
	expect.Equal(t, "bar\nbaz\n", out)
}

const SleeperScript = `#!/bin/bash
touch start
sleep 3
touch finish
`

func TestTimeoutNotReached(t *testing.T) {
	testutils.MediumTest(t)
	dir := TempDir(t, "skexec_test_")
	defer testutils.RemoveAll(t, dir)
	script := filepath.Join(dir, "sleeper_script.sh")
	WriteScript(t, script, SleeperScript)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	assert.NoError(t, NewExec().Run(&Command{
		Name:    script,
		Dir:     dir,
		Context: ctx,
	}))
	_, err := os.Stat(filepath.Join(dir, "finish"))
	expect.NoError(t, err)
}

func TestTimeoutExceeded(t *testing.T) {
	testutils.MediumTest(t)
	dir := TempDir(t, "skexec_test_")
	defer testutils.RemoveAll(t, dir)
	script := filepath.Join(dir, "sleeper_script.sh")
	WriteScript(t, script, SleeperScript)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := NewExec().Run(&Command{
		Name:    script,
		Dir:     dir,
		Context: ctx,
	})
	expect.Error(t, err)
	expect.Contains(t, err.Error(), "signal: killed")
	_, err = os.Stat(filepath.Join(dir, "start"))
	expect.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, "finish"))
	expect.True(t, os.IsNotExist(err))
}

func TestWithCancel(t *testing.T) {
	testutils.MediumTest(t)
	dir := TempDir(t, "skexec_test_")
	defer testutils.RemoveAll(t, dir)
	script := filepath.Join(dir, "sleeper_script.sh")
	WriteScript(t, script, SleeperScript)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var err error
	done := make(chan struct{}, 1)
	go func() {
		err = NewExec().Run(&Command{
			Name:    script,
			Dir:     dir,
			Context: ctx,
		})
		done <- struct{}{}
	}()
	cancel()
	<-done
	expect.Error(t, err)
	if err != context.Canceled {
		expect.Contains(t, err.Error(), "Command killed")
	}
	_, err = os.Stat(filepath.Join(dir, "finish"))
	expect.True(t, os.IsNotExist(err))
}

func TestInjection(t *testing.T) {
	testutils.SmallTest(t)
	exec := NewExec()
	var actualCommand *Command
	exec.SetRun(func(command *Command) error {
		actualCommand = command
		return nil
	})

	dir := TempDir(t, "skexec_test_")
	defer testutils.RemoveAll(t, dir)
	file := filepath.Join(dir, "ran")
	assert.NoError(t, exec.Run(&Command{
		Name: "touch",
		Args: []string{file},
	}))
	_, err := os.Stat(file)
	expect.True(t, os.IsNotExist(err))

	expect.Equal(t, "touch "+file, DebugString(actualCommand))

	exec.Reset()
	assert.NoError(t, exec.Run(&Command{
		Name: "touch",
		Args: []string{file},
	}))
	_, err = os.Stat(file)
	expect.NoError(t, err)
}

func TestRunSimple(t *testing.T) {
	testutils.SmallTest(t)
	output, err := NewExec().RunSimple(`echo "Hello Go!"`)
	assert.NoError(t, err)
	expect.Equal(t, "\"Hello Go!\"\n", output)
}

func TestRunCwd(t *testing.T) {
	testutils.SmallTest(t)
	output, err := NewExec().RunCwd("/", "pwd")
	assert.NoError(t, err)
	expect.Equal(t, "/\n", output)
}

func TestRunAsync(t *testing.T) {
	testutils.LargeTest(t)
	dir := TempDir(t, "skexec_test_")
	defer testutils.RemoveAll(t, dir)
	script := filepath.Join(dir, "sleeper_script.sh")
	WriteScript(t, script, SleeperScript)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := NewExec().RunAsync(&Command{
		Name:    script,
		Dir:     dir,
		Context: ctx,
	})
	err, done := AsyncResult(ch)
	assert.False(t, done)
	assert.Nil(t, err)
	file := filepath.Join(dir, "finish")
	_, err = os.Stat(file)
	expect.True(t, os.IsNotExist(err))
	time.Sleep(5 * time.Second)
	err, done = AsyncResult(ch)
	assert.True(t, done)
	assert.NoError(t, err)
	_, err = os.Stat(file)
	expect.NoError(t, err)
}

func TestRunAsyncCancel(t *testing.T) {
	testutils.LargeTest(t)
	dir := TempDir(t, "skexec_test_")
	defer testutils.RemoveAll(t, dir)
	script := filepath.Join(dir, "sleeper_script.sh")
	WriteScript(t, script, SleeperScript)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := NewExec().RunAsync(&Command{
		Name:    script,
		Dir:     dir,
		Context: ctx,
	})
	startTime := time.Now()
	for _ = range time.Tick(100 * time.Millisecond) {
		_, err := os.Stat(filepath.Join(dir, "start"))
		if err == nil {
			break
		}
		expect.True(t, os.IsNotExist(err))
		if time.Now().Sub(startTime) > 5*time.Second {
			assert.FailNow(t, "Command did not start within 5 seconds.")
		}
	}
	startedTime := time.Now()
	cancel()
	for _ = range time.Tick(100 * time.Millisecond) {
		err, done := AsyncResult(ch)
		if done {
			assert.Error(t, err)
			break
		}
		if time.Now().Sub(startedTime) > 5*time.Second {
			assert.FailNow(t, "Command did not complete within 5 seconds.")
		}
	}
	_, err := os.Stat(filepath.Join(dir, "finish"))
	expect.True(t, os.IsNotExist(err))
}
