package exec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	expect "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/bazel/external/rules_python"
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
		require.NotNil(t, squashed)
		testString1, testString2 := "foobar", "baz"
		n, err := squashed.Write([]byte(testString1))
		expect.Equal(t, len(testString1), n)
		expect.NoError(t, err)
		n, err = squashed.Write([]byte(testString2))
		expect.Equal(t, len(testString2), n)
		expect.NoError(t, err)
		for _, buffer := range input {
			if buffer != nil {
				expect.Equal(t, testString1+testString2, buffer.String())
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
	require.NoError(t, err)
	w = squashWriters(nil, WriteLog{LogFunc: f})
	_, err = w.Write([]byte("obj"))
	require.NoError(t, err)
	expect.Equal(t, "sameobj", out)
}

func TestBasic(t *testing.T) {
	unittest.BazelOnlyTest(t) // Uses the Bazel-downloaded python3 binary.
	dir, err := os.MkdirTemp("", "exec_test")
	require.NoError(t, err)
	defer RemoveAll(dir)
	file := filepath.Join(dir, "ran")
	prog := fmt.Sprintf("with open(r'%s', 'w') as f: f.write('')", file)
	require.NoError(t, Run(context.Background(), &Command{
		Name: findPython3(t),
		Args: []string{"-c", prog},
	}))
	_, err = os.Stat(file)
	expect.NoError(t, err)
}

func TestEnv(t *testing.T) {
	unittest.BazelOnlyTest(t) // Uses the Bazel-downloaded python3 binary.
	dir, err := os.MkdirTemp("", "exec_test")
	require.NoError(t, err)
	defer RemoveAll(dir)
	file := filepath.Join(dir, "ran")

	err = Run(context.Background(), &Command{
		Name: findPython3(t),
		Args: []string{"-c", `
import os
with open(os.environ['EXEC_TEST_FILE'], 'w') as f:
  f.write('')
`},
		Env:         []string{fmt.Sprintf("EXEC_TEST_FILE=%s", file)},
		InheritPath: true,
	})
	require.NoError(t, err)
	_, err = os.Stat(file)
	expect.NoError(t, err)
}

func TestInheritPath(t *testing.T) {
	unittest.BazelOnlyTest(t) // Uses the Bazel-downloaded python3 binary.
	dir, err := os.MkdirTemp("", "exec_test")
	require.NoError(t, err)
	defer RemoveAll(dir)
	file := filepath.Join(dir, "ran")
	require.NoError(t, Run(context.Background(), &Command{
		Name: findPython3(t),
		Args: []string{"-c", `
import os
with open(os.environ['EXEC_TEST_FILE'], 'w') as f:
  f.write(os.environ['PATH'])
`},
		Env:         []string{fmt.Sprintf("EXEC_TEST_FILE=%s", file)},
		InheritPath: true,
	}))
	contents, err := os.ReadFile(file)
	require.NoError(t, err)
	// Python may append site_packages dir to PATH.
	expect.True(t, strings.Contains(strings.TrimSpace(string(contents)), os.Getenv("PATH")))
}

func TestInheritEnv(t *testing.T) {
	unittest.BazelOnlyTest(t) // Uses the Bazel-downloaded python3 binary.
	dir, err := os.MkdirTemp("", "exec_test")
	require.NoError(t, err)
	defer RemoveAll(dir)
	file := filepath.Join(dir, "ran")
	require.NoError(t, Run(context.Background(), &Command{
		Name: findPython3(t),
		Args: []string{"-c", `
import os
with open(os.environ['EXEC_TEST_FILE'], 'w') as f:
  for var in ('PATH', 'USER', 'PWD', 'HOME'):
    f.write('%s\n' % os.environ.get(var, ''))
`},
		Env: []string{
			fmt.Sprintf("EXEC_TEST_FILE=%s", file),
			fmt.Sprintf("HOME=%s", dir),
		},
		InheritPath: false,
		InheritEnv:  true,
	}))
	contents, err := os.ReadFile(file)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(contents)), "\n")
	require.Equal(t, 4, len(lines))
	// Python may add to PATH.
	expect.True(t, strings.Contains(lines[0], os.Getenv("PATH")))
	expect.Equal(t, os.Getenv("USER"), lines[1])
	expect.Equal(t, os.Getenv("PWD"), lines[2])
	expect.Equal(t, dir, lines[3])
}

func TestDir(t *testing.T) {
	unittest.BazelOnlyTest(t) // Uses the Bazel-downloaded python3 binary.
	dir1, err := os.MkdirTemp("", "exec_test1")
	require.NoError(t, err)
	defer RemoveAll(dir1)
	dir2, err := os.MkdirTemp("", "exec_test2")
	require.NoError(t, err)
	defer RemoveAll(dir2)
	require.NoError(t, Run(context.Background(), &Command{
		Name: findPython3(t),
		Args: []string{"-c", "with open('output.txt', 'w') as f: f.write('Hello World!')"},
		Dir:  dir2,
	}))
	file := filepath.Join(dir2, "output.txt")
	_, err = os.Stat(file)
	expect.NoError(t, err)
}

func TestSimpleIO(t *testing.T) {
	unittest.BazelOnlyTest(t) // Uses the Bazel-downloaded python3 binary.
	inputString := "foo\nbar\nbaz\n"
	output := bytes.Buffer{}
	require.NoError(t, Run(context.Background(), &Command{
		Name:   findPython3(t),
		Args:   []string{"-u", "-c", "import sys; sys.stdout.write(sys.stdin.read()[4:])"},
		Stdin:  bytes.NewReader([]byte(inputString)),
		Stdout: &output,
	}))
	expect.Equal(t, "bar\nbaz\n", output.String())
}

func TestError(t *testing.T) {
	unittest.BazelOnlyTest(t) // Uses the Bazel-downloaded python3 binary.
	dir, err := os.MkdirTemp("", "exec_test")
	require.NoError(t, err)
	defer RemoveAll(dir)
	output := bytes.Buffer{}
	err = Run(context.Background(), &Command{
		Name: findPython3(t),
		Args: []string{"-u", "-c", `
import sys
sys.stderr.write('Error in subprocess!')
sys.exit(123)
`},
		Stderr: &output,
	})
	expect.Error(t, err)
	expect.Contains(t, err.Error(), "exit status 123")
	var exitError *exec.ExitError
	require.True(t, errors.As(err, &exitError))
	require.Equal(t, 123, exitError.ExitCode())
	expect.Contains(t, output.String(), "Error in subprocess!")
}

func TestCombinedOutput(t *testing.T) {
	unittest.BazelOnlyTest(t) // Uses the Bazel-downloaded python3 binary.
	dir, err := os.MkdirTemp("", "exec_test")
	require.NoError(t, err)
	defer RemoveAll(dir)
	combined := bytes.Buffer{}
	require.NoError(t, Run(context.Background(), &Command{
		Name: findPython3(t),
		Args: []string{"-u", "-c", `
import sys
sys.stdout.write('roses')
sys.stderr.write('red')
sys.stdout.write('violets')
sys.stderr.write('blue')
`},
		CombinedOutput: &combined,
	}))
	expect.Equal(t, "rosesredvioletsblue", combined.String())
}

// Previously there was a bug due to code like:
// var outputFile *os.File
//
//	if outputToFile {
//		outputFile = ...
//	}
//
// Run(&Command{... Stdout: outputFile})
// See http://devs.cloudimmunity.com/gotchas-and-common-mistakes-in-go-golang/index.html#nil_in_nil_in_vals
func TestNilIO(t *testing.T) {
	unittest.BazelOnlyTest(t) // Uses the Bazel-downloaded python3 binary.
	inputString := "foo\nbar\nbaz\n"
	require.NoError(t, Run(context.Background(), &Command{
		Name:   findPython3(t),
		Args:   []string{"-u", "-c", "import sys; sys.stdout.write(sys.stdin.read()[4:])"},
		Stdin:  bytes.NewReader([]byte(inputString)),
		Stdout: (*os.File)(nil),
	}))
}

func TestTimeoutNotReached(t *testing.T) {
	unittest.BazelOnlyTest(t) // Uses the Bazel-downloaded python3 binary.
	dir, err := os.MkdirTemp("", "exec_test")
	require.NoError(t, err)
	defer RemoveAll(dir)
	require.NoError(t, Run(context.Background(), &Command{
		Name: findPython3(t),
		Args: []string{"-c", `
import time
time.sleep(3)
with open('ran', 'w') as f:
  f.write('')
`},
		Dir:     dir,
		Timeout: time.Minute,
	}))
	file := filepath.Join(dir, "ran")
	_, err = os.Stat(file)
	expect.NoError(t, err)
}

func TestTimeoutExceeded(t *testing.T) {
	unittest.BazelOnlyTest(t) // Uses the Bazel-downloaded python3 binary.
	dir, err := os.MkdirTemp("", "exec_test")
	require.NoError(t, err)
	defer RemoveAll(dir)
	err = Run(context.Background(), &Command{
		Name: findPython3(t),
		Args: []string{"-c", `
import time
time.sleep(3)
with open('ran', 'w') as f:
  f.write('')
`},
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
	ctx := NewContext(context.Background(), func(ctx context.Context, command *Command) error {
		actualCommand = command
		return nil
	})

	dir, err := os.MkdirTemp("", "exec_test")
	require.NoError(t, err)
	defer RemoveAll(dir)
	file := filepath.Join(dir, "ran")
	require.NoError(t, Run(ctx, &Command{
		Name: "touch",
		Args: []string{file},
	}))
	_, err = os.Stat(file)
	expect.True(t, os.IsNotExist(err))

	expect.Equal(t, "touch "+file, DebugString(actualCommand))
}

func TestRunSimple(t *testing.T) {
	cmd := "echo Hello Go!"
	if runtime.GOOS == "windows" {
		cmd = "cmd /C " + cmd
	}
	output, err := RunSimple(context.Background(), cmd)
	require.NoError(t, err)
	expect.Equal(t, "Hello Go!", strings.TrimSpace(output))
}

func TestRunCwd(t *testing.T) {
	unittest.BazelOnlyTest(t) // Uses the Bazel-downloaded python3 binary.
	dir, err := os.MkdirTemp("", "exec_test")
	require.NoError(t, err)
	defer RemoveAll(dir)
	output, err := RunCwd(context.Background(), dir, findPython3(t), "-u", "-c", "import os; print(os.getcwd())")
	require.NoError(t, err)
	expectPath, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	// On Windows, Python capitalizes the drive letter while Go does not.
	expect.Equal(t, strings.ToLower(expectPath+"\n"), strings.ToLower(output))
}

func TestCommandCollector(t *testing.T) {
	mock := CommandCollector{}
	ctx := NewContext(context.Background(), mock.Run)
	require.NoError(t, Run(ctx, &Command{
		Name: "touch",
		Args: []string{"foobar"},
	}))
	require.NoError(t, Run(ctx, &Command{
		Name: "echo",
		Args: []string{"Hello Go!"},
	}))
	commands := mock.Commands()
	require.Len(t, commands, 2)
	expect.Equal(t, "touch foobar", DebugString(commands[0]))
	expect.Equal(t, "echo Hello Go!", DebugString(commands[1]))
	mock.ClearCommands()
	inputString := "foo\nbar\nbaz\n"
	output := bytes.Buffer{}
	require.NoError(t, Run(ctx, &Command{
		Name:   "grep",
		Args:   []string{"-e", "^ba"},
		Stdin:  bytes.NewReader([]byte(inputString)),
		Stdout: &output,
	}))
	commands = mock.Commands()
	require.Len(t, commands, 1)
	expect.Equal(t, "grep -e ^ba", DebugString(commands[0]))
	actualInput, err := io.ReadAll(commands[0].Stdin)
	require.NoError(t, err)
	expect.Equal(t, inputString, string(actualInput))
	expect.Equal(t, &output, commands[0].Stdout)
}

func TestRunCommand(t *testing.T) {
	unittest.BazelOnlyTest(t) // Uses the Bazel-downloaded python3 binary.
	ctx := context.Background()
	// Without a thread-safe io.Writer for Command.CombinedOutput, this test
	// fails "go test -race" and the output does not consistently match the
	// expectation.
	buf := &bytes.Buffer{}
	output, err := RunCommand(ctx, &Command{
		Name:   findPython3(t),
		Args:   []string{"-u", "-c", "print('hello world')"},
		Stdout: buf,
	})
	require.NoError(t, err)
	require.Equal(t, "hello world\n", output)
	require.Equal(t, output, buf.String())
}

func findPython3(t *testing.T) string {
	python3, err := rules_python.FindPython3()
	require.NoError(t, err)
	return python3
}

func TestMergeEnv(t *testing.T) {
	tc := []struct {
		a      []string
		b      []string
		expect []string
	}{
		// Unrelated variables both show up.
		{
			expect: []string{"a=a", "b=b"},
			a:      []string{"a=a"},
			b:      []string{"b=b"},
		},
		// The second env takes precedence over the first.
		{
			expect: []string{"k=v2"},
			a:      []string{"k=v1"},
			b:      []string{"k=v2"},
		},

		// PATH gets special treatment.

		// If only one is specified, it gets preserved.
		{
			expect: []string{"PATH=p2"},
			a:      []string{},
			b:      []string{"PATH=p2"},
		},
		{
			expect: []string{"PATH=p1"},
			a:      []string{"PATH=p1"},
			b:      []string{},
		},
		// The second env takes precedence over the first.
		{
			expect: []string{"PATH=p2"},
			a:      []string{"PATH=p1"},
			b:      []string{"PATH=p2"},
		},
		// ... even if the second env defines it to be empty.
		{
			expect: []string{"PATH="},
			a:      []string{"PATH=p1"},
			b:      []string{"PATH="},
		},
		// If provided, PATH_PLACEHOLDER gets replaced by PATH from the first.
		{
			expect: []string{"PATH=p1:p2"},
			a:      []string{"PATH=p1"},
			b:      []string{fmt.Sprintf("PATH=%s:p2", PathPlaceholder)},
		},
		{
			expect: []string{"PATH=p2:p1"},
			a:      []string{"PATH=p1"},
			b:      []string{fmt.Sprintf("PATH=p2:%s", PathPlaceholder)},
		},
		// There's no good reason to do this, but it would work.
		{
			expect: []string{"PATH=p1:p1"},
			a:      []string{"PATH=p1"},
			b:      []string{fmt.Sprintf("PATH=%s:%s", PathPlaceholder, PathPlaceholder)},
		},
	}

	for _, c := range tc {
		require.Equal(t, c.expect, MergeEnv(c.a, c.b))
	}
}
