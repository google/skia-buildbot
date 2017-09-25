package depot_tools

func TestExtractDEPS(t *testing.T) {
	testutils.MediumTest(t)

	tr := util.NewTempRepo()
	defer tr.Cleanup()

	r, err := NewGitInfo(filepath.Join(tr.Dir, "testrepo"), false, false)
	assert.NoError(t, err)
	extractRegEx, err := regexp.Compile("^.*'skia_revision'.*:.*'([0-9a-f]+)'.*$")
	assert.NoError(t, err)

	// Check for a valid git hash that has a DEPS file.
	assert.Equal(t, "f4b9bf7d9e688f1afedcf4a960a31582ddb56f4a", r.GetDEPSCommit("4ede5bf7c9cc9eccbea0e7c088e47ab8b70aa9a8", extractRegEx))

	// Check an invalid git hash.
	assert.Equal(t, "", r.GetDEPSCommit("invalid-sha-example", extractRegEx))

	// Check a valid git hash that has no DEPS file.
	assert.Equal(t, "", r.GetDEPSCommit("8652a6df7dc8a7e6addee49f6ed3c2308e36bd18", extractRegEx))
}

