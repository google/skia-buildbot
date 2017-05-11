package skexec_testutils

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"

	"go.skia.org/infra/go/skexec"
	"go.skia.org/infra/go/testutils"

	expect "github.com/stretchr/testify/assert"
	assert "github.com/stretchr/testify/require"
)

func TestRunCwd(t *testing.T) {
	testutils.SmallTest(t)
	output := RunCwd(t, "/", "pwd")
	expect.Equal(t, "/\n", output)
}

func TestMockCommandCollector(t *testing.T) {
	testutils.SmallTest(t)
	exec := skexec.NewExec()
	mock := Mock{}
	exec.SetRun(mock.Run)
	defer exec.Reset()
	expect.NoError(t, exec.Run(&skexec.Command{
		Name: "touch",
		Args: []string{"foobar"},
	}))
	out, err := exec.GetOutput(&skexec.Command{
		Name: "echo",
		Args: []string{"Hello Go!"},
	})
	expect.NoError(t, err)
	expect.Equal(t, "", out)
	commands := mock.Commands()
	assert.Len(t, commands, 2)
	expect.Equal(t, "touch foobar", skexec.DebugString(commands[0]))
	expect.Equal(t, "echo Hello Go!", skexec.DebugString(commands[1]))
	mock.ClearCommands()
	inputString := "foo\nbar\nbaz\n"
	output := bytes.Buffer{}
	expect.NoError(t, exec.Run(&skexec.Command{
		Name:   "grep",
		Args:   []string{"-e", "^ba"},
		Stdin:  bytes.NewReader([]byte(inputString)),
		Stdout: &output,
	}))
	commands = mock.Commands()
	assert.Len(t, commands, 1)
	expect.Equal(t, "grep -e ^ba", skexec.DebugString(commands[0]))
	actualInput, err := ioutil.ReadAll(commands[0].Stdin)
	expect.NoError(t, err)
	expect.Equal(t, inputString, string(actualInput))
	expect.Equal(t, &output, commands[0].Stdout)
}

func TestMockAddRule(t *testing.T) {
	testutils.SmallTest(t)
	exec := skexec.NewExec()
	mock := &Mock{}
	exec.SetRun(mock.Run)
	defer exec.Reset()
	mock.AddRule("echo [hH]ello", "Hello\n", nil)
	mock.AddRule("^touch /tmp/bar$", "", fmt.Errorf("baz"))
	mock.AddRule("whoami", "janedoe", fmt.Errorf("We don't know either."))
	out, err := exec.GetOutput(&skexec.Command{
		Name: "echo",
		Args: []string{"hello"},
	})
	expect.NoError(t, err)
	expect.Equal(t, "Hello\n", out)
	expect.NoError(t, exec.Run(&skexec.Command{
		Name: "touch",
		Args: []string{"/tmp/foo"},
	}))
	err = exec.Run(&skexec.Command{
		Name: "touch",
		Args: []string{"/tmp/bar"},
	})
	assert.Error(t, err)
	expect.Contains(t, err.Error(), "baz")
	out, err = exec.GetOutput(&skexec.Command{
		Name: "whoami",
	})
	assert.Error(t, err)
	expect.Contains(t, err.Error(), "We don't know either.")
	expect.Equal(t, "janedoe", out)
}

func TestMockDefaultRun(t *testing.T) {
	testutils.SmallTest(t)
	exec := skexec.NewExec()
	mock := Mock{}
	exec.SetRun(mock.Run)
	defer exec.Reset()
	expectCall := false
	var nextError error
	mock.SetDefaultRun(func(cmd *skexec.Command) error {
		expect.True(t, expectCall)
		expectCall = false
		commands := mock.Commands()
		expect.Equal(t, cmd, commands[len(commands)-1])
		if cmd.Stdout != nil {
			_, _ = cmd.Stdout.Write([]byte(skexec.DebugString(cmd)))
		}
		return nextError
	})

	nextError = fmt.Errorf("foobar is fubar")
	expectCall = true
	expect.Equal(t, nextError, exec.Run(&skexec.Command{
		Name: "touch",
		Args: []string{"foobar"},
	}))

	nextError = nil
	expectCall = true
	out, err := exec.GetOutput(&skexec.Command{
		Name: "echo",
		Args: []string{"Hello Go!"},
	})
	expect.NoError(t, err)
	expect.Equal(t, "echo Hello Go!", out)

	mock.AddRule("touch /tmp/bar", "", fmt.Errorf("baz"))

	expectCall = true
	out, err = exec.GetOutput(&skexec.Command{
		Name: "touch",
		Args: []string{"/tmp/foo"},
	})
	expect.NoError(t, err)
	expect.Equal(t, "touch /tmp/foo", out)

	expectCall = false
	err = exec.Run(&skexec.Command{
		Name: "touch",
		Args: []string{"/tmp/bar"},
	})
	assert.Error(t, err)
	expect.Contains(t, err.Error(), "baz")
}
