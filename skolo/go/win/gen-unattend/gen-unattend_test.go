package main

import (
	"bytes"
	"html/template"
	"io"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"testing"

	"go.skia.org/infra/go/config"
	"go.skia.org/infra/go/testutils"

	expect "github.com/stretchr/testify/assert"
	assert "github.com/stretchr/testify/require"
)

func TestParseExampleConfig(t *testing.T) {
	testutils.MediumTest(t)
	_, filename, _, _ := runtime.Caller(0)

	devices := DevicesConfig{}
	config.MustParseConfigFile(filepath.Join(filepath.Dir(filename), "example-devices.json5"), "", &devices)
	expect.Len(t, devices.Devices, 3)
	expect.Equal(t, "unattend-skiabot.xml", devices.Devices["skia-e-win-010"].Unattend)
	expect.Equal(t, "unattend-skiabot.xml", devices.Devices["skia-e-win-011"].Unattend)
	expect.Equal(t, "unattend-skiabot.xml", devices.Devices["skia-e-win-012"].Unattend)

	globalVars := GlobalVars{}
	config.MustParseConfigFile(filepath.Join(filepath.Dir(filename), "example-vars.json5"), "", &globalVars)
	expect.Equal(t, "topsecret", globalVars.AdminPassword)
	expect.Equal(t, "classified", globalVars.ChromeBotPassword)
}

func TestValidateTemplates(t *testing.T) {
	testutils.MediumTest(t)
	_, filename, _, _ := runtime.Caller(0)
	tp, err := template.ParseGlob(filepath.Join(filepath.Dir(filename), "../../../win/templates/*.xml"))
	assert.NoError(t, err)
	assert.NotNil(t, tp)
}

const (
	simpleTemplate = `<xml>
  <setup>{{.DeviceName}}</setup>
  <plz>{{.AdminPassword}}</plz>
  <kthxbai/>
</xml>`
)

func TestRunTemplates(t *testing.T) {
	testutils.SmallTest(t)
	tp, err := template.New("simple.xml").Parse(simpleTemplate)
	assert.NoError(t, err)

	devices := DevicesConfig{
		Devices: map[string]DeviceConfig{
			"HAL": {
				Unattend: "simple.xml",
			},
			"Skynet": {
				Unattend: "simple.xml",
			},
		},
	}
	vars := GlobalVars{
		AdminPassword: "notverysecret",
	}
	actual, err := runTemplates(devices, vars, tp)
	assert.NoError(t, err)
	expect.Len(t, actual, 2)
	{
		actual, ok := actual["unattend-HAL.xml"]
		assert.True(t, ok)
		assert.Equal(t, `<xml>
  <setup>HAL</setup>
  <plz>notverysecret</plz>
  <kthxbai/>
</xml>`, string(actual))
	}
	{
		actual, ok := actual["unattend-Skynet.xml"]
		assert.True(t, ok)
		assert.Equal(t, `<xml>
  <setup>Skynet</setup>
  <plz>notverysecret</plz>
  <kthxbai/>
</xml>`, string(actual))
	}
}

func TestRunTemplatesUnknownTemplate(t *testing.T) {
	testutils.SmallTest(t)
	tp, err := template.New("simple.xml").Parse(simpleTemplate)
	assert.NoError(t, err)
	devices := DevicesConfig{
		Devices: map[string]DeviceConfig{
			"Skynet": {
				Unattend: "sentient.xml",
			},
		},
	}
	vars := GlobalVars{}
	_, err = runTemplates(devices, vars, tp)
	assert.Error(t, err)
	assert.Equal(t, `For device "Skynet": no such unattend template "sentient.xml".`, err.Error())
}

func TestRunTemplatesBadTemplate(t *testing.T) {
	testutils.SmallTest(t)
	tp, err := template.New("bad.xml").Parse(`<xml>{{.BadField}}</xml>`)
	assert.NoError(t, err)
	devices := DevicesConfig{
		Devices: map[string]DeviceConfig{
			"Skynet": {
				Unattend: "bad.xml",
			},
		},
	}
	vars := GlobalVars{}
	_, err = runTemplates(devices, vars, tp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `For device "Skynet": error executing template:`)
	assert.Contains(t, err.Error(), "can't evaluate field BadField")
}

func TestComputeDiffs(t *testing.T) {
	testutils.MediumTest(t)
	tmpdir, err := ioutil.TempDir("", "TestComputeDiffs")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmpdir)

	expected := map[string][]byte{
		"B": []byte("B"),
		"C": []byte("C"),
		"E": []byte("E"),
		"F": []byte("F"),
	}
	actual, err := computeDiffs(expected, tmpdir)
	assert.NoError(t, err)
	t.Logf("%#v", actual)
	testutils.AssertDeepEqual(t, diffs{
		toCreate: []string{"B", "C", "E", "F"},
	}, actual)

	testutils.WriteFile(t, filepath.Join(tmpdir, "B"), "B")
	testutils.WriteFile(t, filepath.Join(tmpdir, "E"), "E")
	actual, err = computeDiffs(expected, tmpdir)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, diffs{
		toCreate: []string{"C", "F"},
	}, actual)

	testutils.WriteFile(t, filepath.Join(tmpdir, "B"), "b")
	testutils.WriteFile(t, filepath.Join(tmpdir, "F"), "f")
	actual, err = computeDiffs(expected, tmpdir)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, diffs{
		toCreate: []string{"C"},
		toModify: []string{"B", "F"},
	}, actual)

	testutils.WriteFile(t, filepath.Join(tmpdir, "A"), "a")
	testutils.WriteFile(t, filepath.Join(tmpdir, "D"), "d")
	actual, err = computeDiffs(expected, tmpdir)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, diffs{
		toCreate: []string{"C"},
		toModify: []string{"B", "F"},
		toDelete: []string{"A", "D"},
	}, actual)

	testutils.Remove(t, filepath.Join(tmpdir, "A"))
	testutils.WriteFile(t, filepath.Join(tmpdir, "B"), "B")
	testutils.WriteFile(t, filepath.Join(tmpdir, "C"), "C")
	testutils.Remove(t, filepath.Join(tmpdir, "D"))
	testutils.WriteFile(t, filepath.Join(tmpdir, "F"), "F")
	actual, err = computeDiffs(expected, tmpdir)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, diffs{}, actual)
}

// Call confirmDiffsImpl with the given diffs, outDir of "/foo/bar", given assumeYes, and stdin
// reading from input; return the stdout, and fail the test on abort.
func runConfirmDiffs(t *testing.T, d diffs, assumeYes bool, input string) string {
	buf := bytes.Buffer{}
	assert.NoError(t, confirmDiffsImpl(d, "/foo/bar", assumeYes, bytes.NewReader([]byte(input)), &buf))
	return buf.String()
}

func TestConfirmDiffsNoDiffs(t *testing.T) {
	testutils.SmallTest(t)
	out := runConfirmDiffs(t, diffs{}, true, "")
	expect.Equal(t, "No changes.\n", out)
}

func TestConfirmDiffsCreate(t *testing.T) {
	testutils.SmallTest(t)
	out := runConfirmDiffs(t, diffs{
		toCreate: []string{"A", "B"},
	}, true, "")
	expect.Equal(t, `Create 2 file(s):
	/foo/bar/A
	/foo/bar/B

`, out)
}

func TestConfirmDiffsModify(t *testing.T) {
	testutils.SmallTest(t)
	out := runConfirmDiffs(t, diffs{
		toModify: []string{"A", "B", "C"},
	}, true, "")
	expect.Equal(t, `Modify 3 file(s):
	/foo/bar/A
	/foo/bar/B
	/foo/bar/C

`, out)
}

func TestConfirmDiffsSeveral(t *testing.T) {
	testutils.SmallTest(t)
	out := runConfirmDiffs(t, diffs{
		toCreate: []string{"A", "E"},
		toModify: []string{"B", "C", "G"},
		toDelete: []string{"D", "F"},
	}, true, "")
	expect.Equal(t, `Create 2 file(s):
	/foo/bar/A
	/foo/bar/E

Modify 3 file(s):
	/foo/bar/B
	/foo/bar/C
	/foo/bar/G

Delete 2 file(s):
	/foo/bar/D
	/foo/bar/F

`, out)
}

func TestConfirmDiffsPrompt(t *testing.T) {
	testutils.SmallTest(t)
	out := runConfirmDiffs(t, diffs{
		toModify: []string{"A"},
	}, false, "Y\n")
	expect.Equal(t, `Modify 1 file(s):
	/foo/bar/A

Continue? (y/N) `, out)
}

func assertConfirmDiffsAbort(t *testing.T, response string) {
	buf := bytes.Buffer{}
	r, w := io.Pipe()
	errCh := make(chan error, 1)
	go func() {
		errCh <- confirmDiffsImpl(diffs{
			toModify: []string{"A"},
		}, "/foo/bar", false, r, &buf)
	}()
	for !bytes.HasSuffix(buf.Bytes(), []byte("Continue? (y/N) ")) {
		runtime.Gosched()
	}
	// Check that confirmDiffsImpl waits for input.
	assert.Len(t, errCh, 0)
	n, err := w.Write([]byte(response))
	assert.NoError(t, err)
	assert.Equal(t, len(response), n)
	err = <-errCh
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Aborted")
}

func TestConfirmDiffsAbort(t *testing.T) {
	testutils.SmallTest(t)
	assertConfirmDiffsAbort(t, "\n")
	assertConfirmDiffsAbort(t, "n \n")
	assertConfirmDiffsAbort(t, "N \n")
	assertConfirmDiffsAbort(t, "wut?\n")
}

func setupDirContents(t *testing.T, dirContents map[string][]byte) (string, func()) {
	testutils.MediumTest(t)
	tmpdir, err := ioutil.TempDir("", "assertWriteFiles")
	assert.NoError(t, err)

	for file, data := range dirContents {
		testutils.WriteFile(t, filepath.Join(tmpdir, file), string(data))
	}

	return tmpdir, func() {
		testutils.RemoveAll(t, tmpdir)
	}
}

func assertDirContents(t *testing.T, dir string, dirContents map[string][]byte) {
	d, err := computeDiffs(dirContents, dir)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, diffs{}, d)
}

func TestWriteFilesCreate(t *testing.T) {
	init := map[string][]byte{
		"B": []byte("B"),
		"E": []byte("E"),
		"X": []byte("unchanged"),
	}
	tmpdir, cleanup := setupDirContents(t, init)
	defer cleanup()

	input := map[string][]byte{
		"B": []byte("B"),
		"C": []byte("C"),
		"E": []byte("E"),
		"F": []byte("F"),
		"X": []byte("changed"),
	}
	d := diffs{
		toCreate: []string{"C", "F"},
	}
	assert.NoError(t, writeFiles(input, d, tmpdir))

	expected := map[string][]byte{
		"B": []byte("B"),
		"C": []byte("C"),
		"E": []byte("E"),
		"F": []byte("F"),
		"X": []byte("unchanged"),
	}
	assertDirContents(t, tmpdir, expected)
}

func TestWriteFilesModifyCreate(t *testing.T) {
	init := map[string][]byte{
		"B": []byte("b"),
		"E": []byte("E"),
		"F": []byte("f"),
		"X": []byte("unchanged"),
	}
	tmpdir, cleanup := setupDirContents(t, init)
	defer cleanup()

	input := map[string][]byte{
		"B": []byte("B"),
		"C": []byte("C"),
		"E": []byte("E"),
		"F": []byte("F"),
		"X": []byte("changed"),
	}
	d := diffs{
		toCreate: []string{"C"},
		toModify: []string{"B", "F"},
	}
	assert.NoError(t, writeFiles(input, d, tmpdir))

	expected := map[string][]byte{
		"B": []byte("B"),
		"C": []byte("C"),
		"E": []byte("E"),
		"F": []byte("F"),
		"X": []byte("unchanged"),
	}
	assertDirContents(t, tmpdir, expected)
}

func TestWriteFilesDeleteModifyCreate(t *testing.T) {
	init := map[string][]byte{
		"A": []byte("a"),
		"B": []byte("b"),
		"D": []byte("d"),
		"E": []byte("E"),
		"F": []byte("f"),
		"X": []byte("unchanged"),
	}
	tmpdir, cleanup := setupDirContents(t, init)
	defer cleanup()

	input := map[string][]byte{
		"B": []byte("B"),
		"C": []byte("C"),
		"E": []byte("E"),
		"F": []byte("F"),
		"X": []byte("changed"),
	}
	d := diffs{
		toCreate: []string{"C"},
		toModify: []string{"B", "F"},
		toDelete: []string{"A", "D"},
	}
	assert.NoError(t, writeFiles(input, d, tmpdir))

	expected := map[string][]byte{
		"B": []byte("B"),
		"C": []byte("C"),
		"E": []byte("E"),
		"F": []byte("F"),
		"X": []byte("unchanged"),
	}
	assertDirContents(t, tmpdir, expected)
}

func TestWriteFilesNoop(t *testing.T) {
	init := map[string][]byte{
		"X": []byte("unchanged"),
	}
	tmpdir, cleanup := setupDirContents(t, init)
	defer cleanup()

	input := map[string][]byte{
		"B": []byte("B"),
		"C": []byte("C"),
		"E": []byte("E"),
		"F": []byte("F"),
		"X": []byte("changed"),
	}
	d := diffs{}
	assert.NoError(t, writeFiles(input, d, tmpdir))

	expected := map[string][]byte{
		"X": []byte("unchanged"),
	}
	assertDirContents(t, tmpdir, expected)
}
