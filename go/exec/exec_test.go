package exec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	expect "github.com/stretchr/testify/assert"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils/unittest"
)

// Copied from go.skia.org/infra/go/util/util.go to avoid recursive dependency.
func RemoveAll(path string) {
	if err := os.RemoveAll(path); err != nil {
		sklog.Errorf("Failed to RemoveAll(%s): %v", path, err)
	}
}

func TestParseCommand(t *testing.T) {
	unittest.SmallTest(t)
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
	unittest.SmallTest(t)
	expect.Equal(t, nil, squashWriters())
	expect.Equal(t, nil, squashWriters(nil))
	expect.Equal(t, nil, squashWriters(nil, nil))
	expect.Equal(t, nil, squashWriters((*bytes.Buffer)(nil)))
	expect.Equal(t, nil, squashWriters((*bytes.Buffer)(nil), (*os.File)(nil)))
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
	test(&bytes.Buffer{})
	test(&bytes.Buffer{}, &bytes.Buffer{})
	test(&bytes.Buffer{}, nil)
	test(nil, &bytes.Buffer{})
	test(&bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})
	test(&bytes.Buffer{}, nil, nil)
	test(nil, &bytes.Buffer{}, nil)
	test(nil, nil, &bytes.Buffer{})
	test(&bytes.Buffer{}, nil, &bytes.Buffer{})
	// Test with non-pointer io.Writers.
	// expect.Equal returns false for two WriteLogs pointing to the same function, so we test
	// by side-effect instead.
	out := ""
	f := func(format string, args ...interface{}) { out = out + fmt.Sprintf(format, args...) }
	w := squashWriters(WriteLog{LogFunc: f}, (*os.File)(nil))
	_, err := w.Write([]byte("same"))
	assert.NoError(t, err)
	w = squashWriters(nil, WriteLog{LogFunc: f})
	_, err = w.Write([]byte("obj"))
	assert.NoError(t, err)
	expect.Equal(t, "sameobj", out)
}

func TestBasic(t *testing.T) {
	unittest.SmallTest(t)
	dir, err := ioutil.TempDir("", "exec_test")
	assert.NoError(t, err)
	defer RemoveAll(dir)
	file := filepath.Join(dir, "ran")
	assert.NoError(t, Run(context.Background(), &Command{
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
	unittest.SmallTest(t)
	dir, err := ioutil.TempDir("", "exec_test")
	assert.NoError(t, err)
	defer RemoveAll(dir)
	script := filepath.Join(dir, "simple_script.sh")
	assert.NoError(t, WriteScript(script, SimpleScript))
	file := filepath.Join(dir, "ran")
	assert.NoError(t, Run(context.Background(), &Command{
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
	unittest.SmallTest(t)
	dir, err := ioutil.TempDir("", "exec_test")
	assert.NoError(t, err)
	defer RemoveAll(dir)
	script := filepath.Join(dir, "path_script.sh")
	assert.NoError(t, WriteScript(script, PathScript))
	file := filepath.Join(dir, "ran")
	assert.NoError(t, Run(context.Background(), &Command{
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
	unittest.SmallTest(t)
	dir, err := ioutil.TempDir("", "exec_test")
	assert.NoError(t, err)
	defer RemoveAll(dir)
	script := filepath.Join(dir, "path_script.sh")
	assert.NoError(t, WriteScript(script, EnvScript))
	file := filepath.Join(dir, "ran")
	assert.NoError(t, Run(context.Background(), &Command{
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
	assert.Equal(t, 5, len(lines))
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
	unittest.SmallTest(t)
	dir1, err := ioutil.TempDir("", "exec_test1")
	assert.NoError(t, err)
	defer RemoveAll(dir1)
	script := filepath.Join(dir1, "hello_script.sh")
	assert.NoError(t, WriteScript(script, HelloScript))
	dir2, err := ioutil.TempDir("", "exec_test2")
	assert.NoError(t, err)
	defer RemoveAll(dir2)
	assert.NoError(t, Run(context.Background(), &Command{
		Name: script,
		Dir:  dir2,
	}))
	file := filepath.Join(dir2, "output.txt")
	_, err = os.Stat(file)
	expect.NoError(t, err)
}

func TestSimpleIO(t *testing.T) {
	unittest.SmallTest(t)
	inputString := "foo\nbar\nbaz\n"
	output := bytes.Buffer{}
	assert.NoError(t, Run(context.Background(), &Command{
		Name:   "grep",
		Args:   []string{"-e", "^ba"},
		Stdin:  bytes.NewReader([]byte(inputString)),
		Stdout: &output,
	}))
	expect.Equal(t, "bar\nbaz\n", string(output.Bytes()))
}

func TestError(t *testing.T) {
	unittest.SmallTest(t)
	dir, err := ioutil.TempDir("", "exec_test")
	assert.NoError(t, err)
	defer RemoveAll(dir)
	output := bytes.Buffer{}
	err = Run(context.Background(), &Command{
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
	unittest.SmallTest(t)
	dir, err := ioutil.TempDir("", "exec_test")
	assert.NoError(t, err)
	defer RemoveAll(dir)
	script := filepath.Join(dir, "combined_output_script.sh")
	assert.NoError(t, WriteScript(script, CombinedOutputScript))
	combined := bytes.Buffer{}
	assert.NoError(t, Run(context.Background(), &Command{
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
	unittest.SmallTest(t)
	inputString := "foo\nbar\nbaz\n"
	assert.NoError(t, Run(context.Background(), &Command{
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
	unittest.MediumTest(t)
	dir, err := ioutil.TempDir("", "exec_test")
	assert.NoError(t, err)
	defer RemoveAll(dir)
	script := filepath.Join(dir, "sleeper_script.sh")
	assert.NoError(t, WriteScript(script, SleeperScript))
	assert.NoError(t, Run(context.Background(), &Command{
		Name:    script,
		Dir:     dir,
		Timeout: time.Minute,
	}))
	file := filepath.Join(dir, "ran")
	_, err = os.Stat(file)
	expect.NoError(t, err)
}

func TestTimeoutExceeded(t *testing.T) {
	unittest.MediumTest(t)
	dir, err := ioutil.TempDir("", "exec_test")
	assert.NoError(t, err)
	defer RemoveAll(dir)
	script := filepath.Join(dir, "sleeper_script.sh")
	assert.NoError(t, WriteScript(script, SleeperScript))
	err = Run(context.Background(), &Command{
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
	unittest.SmallTest(t)
	var actualCommand *Command
	ctx := NewContext(context.Background(), func(command *Command) error {
		actualCommand = command
		return nil
	})

	dir, err := ioutil.TempDir("", "exec_test")
	assert.NoError(t, err)
	defer RemoveAll(dir)
	file := filepath.Join(dir, "ran")
	assert.NoError(t, Run(ctx, &Command{
		Name: "touch",
		Args: []string{file},
	}))
	_, err = os.Stat(file)
	expect.True(t, os.IsNotExist(err))

	expect.Equal(t, "touch "+file, DebugString(actualCommand))
}

func TestRunSimple(t *testing.T) {
	unittest.SmallTest(t)
	output, err := RunSimple(context.Background(), `echo "Hello Go!"`)
	assert.NoError(t, err)
	expect.Equal(t, "\"Hello Go!\"\n", output)
}

func TestRunCwd(t *testing.T) {
	unittest.SmallTest(t)
	output, err := RunCwd(context.Background(), "/", "pwd")
	assert.NoError(t, err)
	expect.Equal(t, "/\n", output)
}

func TestCommandCollector(t *testing.T) {
	unittest.SmallTest(t)
	mock := CommandCollector{}
	ctx := NewContext(context.Background(), mock.Run)
	assert.NoError(t, Run(ctx, &Command{
		Name: "touch",
		Args: []string{"foobar"},
	}))
	assert.NoError(t, Run(ctx, &Command{
		Name: "echo",
		Args: []string{"Hello Go!"},
	}))
	commands := mock.Commands()
	assert.Len(t, commands, 2)
	expect.Equal(t, "touch foobar", DebugString(commands[0]))
	expect.Equal(t, "echo Hello Go!", DebugString(commands[1]))
	mock.ClearCommands()
	inputString := "foo\nbar\nbaz\n"
	output := bytes.Buffer{}
	assert.NoError(t, Run(ctx, &Command{
		Name:   "grep",
		Args:   []string{"-e", "^ba"},
		Stdin:  bytes.NewReader([]byte(inputString)),
		Stdout: &output,
	}))
	commands = mock.Commands()
	assert.Len(t, commands, 1)
	expect.Equal(t, "grep -e ^ba", DebugString(commands[0]))
	actualInput, err := ioutil.ReadAll(commands[0].Stdin)
	assert.NoError(t, err)
	expect.Equal(t, inputString, string(actualInput))
	expect.Equal(t, &output, commands[0].Stdout)
}

func TestMockRun(t *testing.T) {
	unittest.SmallTest(t)
	mock := MockRun{}
	ctx := NewContext(context.Background(), mock.Run)
	mock.AddRule("touch /tmp/bar", fmt.Errorf("baz"))
	assert.NoError(t, Run(ctx, &Command{
		Name: "touch",
		Args: []string{"/tmp/foo"},
	}))
	err := Run(ctx, &Command{
		Name: "touch",
		Args: []string{"/tmp/bar"},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "baz")
}

func TestRunCommand(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()
	// Without a thread-safe io.Writer for Command.CombinedOutput, this test
	// fails "go test -race" and the output does not consistently match the
	// expectation.
	buf := &bytes.Buffer{}
	output, err := RunCommand(ctx, &Command{
		Name:   "python",
		Args:   []string{"-c", "print 'hello world'"},
		Stdout: buf,
	})
	assert.NoError(t, err)
	assert.Equal(t, "hello world\n", output)
	assert.Equal(t, output, buf.String())
}
