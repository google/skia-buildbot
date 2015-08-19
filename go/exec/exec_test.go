package exec

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/skia-dev/glog"
	expect "github.com/stretchr/testify/assert"
	assert "github.com/stretchr/testify/require"
)

// Copied from go.skia.org/infra/go/util/util.go to avoid recursive dependency.
func RemoveAll(path string) {
	if err := os.RemoveAll(path); err != nil {
		glog.Errorf("Failed to RemoveAll(%s): %v", path, err)
	}
}

func TestParseCommand(t *testing.T) {
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

func TestSquashWriters(t *testing.T) {
	test := func(input ...*bytes.Buffer) {
		writers := make([]io.Writer, len(input))
		for i, buffer := range input {
			if buffer != nil {
				writers[i] = buffer
			}
		}
		squashed := squashWriters(writers...)
		assert.NotNil(t, squashed)
		testString1, testString2 := "foobar", "baz"
		n, err := squashed.Write([]byte(testString1))
		expect.Equal(t, len(testString1), n)
		expect.NoError(t, err)
		n, err = squashed.Write([]byte(testString2))
		expect.Equal(t, len(testString2), n)
		expect.NoError(t, err)
		for _, buffer := range input {
			if buffer != nil {
				expect.Equal(t, testString1+testString2, string(buffer.Bytes()))
			}
		}
	}
	expect.Equal(t, nil, squashWriters())
	expect.Equal(t, nil, squashWriters(nil))
	expect.Equal(t, nil, squashWriters(nil, nil))
	test(&bytes.Buffer{})
	test(&bytes.Buffer{}, &bytes.Buffer{})
	test(&bytes.Buffer{}, nil)
	test(nil, &bytes.Buffer{})
	test(&bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})
	test(&bytes.Buffer{}, nil, nil)
	test(nil, &bytes.Buffer{}, nil)
	test(nil, nil, &bytes.Buffer{})
	test(&bytes.Buffer{}, nil, &bytes.Buffer{})
}

func TestBasic(t *testing.T) {
	dir, err := ioutil.TempDir("", "exec_test")
	assert.NoError(t, err)
	defer RemoveAll(dir)
	file := filepath.Join(dir, "ran")
	assert.NoError(t, Run(&Command{
		Name: "touch",
		Args: []string{file},
	}))
	_, err = os.Stat(file)
	expect.NoError(t, err)
}

func WriteScript(path, script string) error {
	return ioutil.WriteFile(path, []byte(script), 0777)
}

const SimpleScript = `#!/bin/bash
touch "${EXEC_TEST_FILE}"
`

func TestEnv(t *testing.T) {
	dir, err := ioutil.TempDir("", "exec_test")
	assert.NoError(t, err)
	defer RemoveAll(dir)
	script := filepath.Join(dir, "simple_script.sh")
	assert.NoError(t, WriteScript(script, SimpleScript))
	file := filepath.Join(dir, "ran")
	assert.NoError(t, Run(&Command{
		Name: script,
		Env:  []string{fmt.Sprintf("EXEC_TEST_FILE=%s", file)},
	}))
	_, err = os.Stat(file)
	expect.NoError(t, err)
}

const PathScript = `#!/bin/bash
echo "${PATH}" > "${EXEC_TEST_FILE}"
`

func TestInheritPath(t *testing.T) {
	dir, err := ioutil.TempDir("", "exec_test")
	assert.NoError(t, err)
	defer RemoveAll(dir)
	script := filepath.Join(dir, "path_script.sh")
	assert.NoError(t, WriteScript(script, PathScript))
	file := filepath.Join(dir, "ran")
	assert.NoError(t, Run(&Command{
		Name:        script,
		Env:         []string{fmt.Sprintf("EXEC_TEST_FILE=%s", file)},
		InheritPath: true,
	}))
	contents, err := ioutil.ReadFile(file)
	assert.NoError(t, err)
	expect.Equal(t, os.Getenv("PATH"), strings.TrimSpace(string(contents)))
}

const HelloScript = `#!/bin/bash
echo "Hello World!" > output.txt
`

func TestDir(t *testing.T) {
	dir1, err := ioutil.TempDir("", "exec_test1")
	assert.NoError(t, err)
	defer RemoveAll(dir1)
	script := filepath.Join(dir1, "hello_script.sh")
	assert.NoError(t, WriteScript(script, HelloScript))
	dir2, err := ioutil.TempDir("", "exec_test2")
	assert.NoError(t, err)
	defer RemoveAll(dir2)
	assert.NoError(t, Run(&Command{
		Name: script,
		Dir:  dir2,
	}))
	file := filepath.Join(dir2, "output.txt")
	_, err = os.Stat(file)
	expect.NoError(t, err)
}

func TestSimpleIO(t *testing.T) {
	inputString := "foo\nbar\nbaz\n"
	output := bytes.Buffer{}
	assert.NoError(t, Run(&Command{
		Name:   "grep",
		Args:   []string{"-e", "^ba"},
		Stdin:  bytes.NewReader([]byte(inputString)),
		Stdout: &output,
	}))
	expect.Equal(t, "bar\nbaz\n", string(output.Bytes()))
}

func TestError(t *testing.T) {
	dir, err := ioutil.TempDir("", "exec_test")
	assert.NoError(t, err)
	defer RemoveAll(dir)
	output := bytes.Buffer{}
	err = Run(&Command{
		Name: "cp",
		Args: []string{filepath.Join(dir, "doesnt_exist"),
			filepath.Join(dir, "dest")},
		Stderr: &output,
	})
	expect.Error(t, err)
	expect.Contains(t, err.Error(), "exit status 1")
	expect.Contains(t, string(output.Bytes()), "No such file or directory")
}

const CombinedOutputScript = `#!/bin/bash
echo "roses"
>&2 echo "red"
echo "violets"
>&2 echo "blue"
`

func TestCombinedOutput(t *testing.T) {
	dir, err := ioutil.TempDir("", "exec_test")
	assert.NoError(t, err)
	defer RemoveAll(dir)
	script := filepath.Join(dir, "combined_output_script.sh")
	assert.NoError(t, WriteScript(script, CombinedOutputScript))
	combined := bytes.Buffer{}
	assert.NoError(t, Run(&Command{
		Name:           script,
		CombinedOutput: &combined,
	}))
	expect.Equal(t, "roses\nred\nviolets\nblue\n", string(combined.Bytes()))
}

// Previously there was a bug due to code like:
// var outputFile *os.File
// if outputToFile {
// 	outputFile = ...
// }
// Run(&Command{... Stdout: outputFile})
// See http://devs.cloudimmunity.com/gotchas-and-common-mistakes-in-go-golang/index.html#nil_in_nil_in_vals
func TestNilIO(t *testing.T) {
	inputString := "foo\nbar\nbaz\n"
	assert.NoError(t, Run(&Command{
		Name:   "grep",
		Args:   []string{"-e", "^ba"},
		Stdin:  bytes.NewReader([]byte(inputString)),
		Stdout: (*os.File)(nil),
	}))
}

const SleeperScript = `#!/bin/bash
sleep 3
touch ran
`

func TestTimeoutNotReached(t *testing.T) {
	dir, err := ioutil.TempDir("", "exec_test")
	assert.NoError(t, err)
	defer RemoveAll(dir)
	script := filepath.Join(dir, "sleeper_script.sh")
	assert.NoError(t, WriteScript(script, SleeperScript))
	assert.NoError(t, Run(&Command{
		Name:    script,
		Dir:     dir,
		Timeout: time.Minute,
	}))
	file := filepath.Join(dir, "ran")
	_, err = os.Stat(file)
	expect.NoError(t, err)
}

func TestTimeoutExceeded(t *testing.T) {
	dir, err := ioutil.TempDir("", "exec_test")
	assert.NoError(t, err)
	defer RemoveAll(dir)
	script := filepath.Join(dir, "sleeper_script.sh")
	assert.NoError(t, WriteScript(script, SleeperScript))
	err = Run(&Command{
		Name:    script,
		Dir:     dir,
		Timeout: time.Second,
	})
	expect.Error(t, err)
	expect.Contains(t, err.Error(), "Command killed")
	file := filepath.Join(dir, "ran")
	_, err = os.Stat(file)
	expect.True(t, os.IsNotExist(err))
}

func TestInjection(t *testing.T) {
	var actualCommand *Command
	SetRunForTesting(func(command *Command) error {
		actualCommand = command
		return nil
	})
	defer SetRunForTesting(DefaultRun)

	dir, err := ioutil.TempDir("", "exec_test")
	assert.NoError(t, err)
	defer RemoveAll(dir)
	file := filepath.Join(dir, "ran")
	assert.NoError(t, Run(&Command{
		Name: "touch",
		Args: []string{file},
	}))
	_, err = os.Stat(file)
	expect.True(t, os.IsNotExist(err))

	expect.Equal(t, "touch", actualCommand.Name)
	expect.Equal(t, 1, len(actualCommand.Args))
	expect.Equal(t, file, actualCommand.Args[0])
}

func TestRunSimple(t *testing.T) {
	output, err := RunSimple(`echo "Hello Go!"`)
	assert.NoError(t, err)
	expect.Equal(t, "\"Hello Go!\"\n", output)
}
