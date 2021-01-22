package main

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/gold-client/go/goldclient"
)

func TestImgTest_StreamingPassFail_MultipleSeparateUploads(t *testing.T) {
	unittest.MediumTest(t)

	workDir := t.TempDir()
	setupAuthWithGSUtil(t, workDir)
	td, err := testutils.TestDataDir()
	require.NoError(t, err)

	env := imgTest{
		commitHash:              "1234567890123456789012345678901234567890",
		corpus:                  "my_corpus",
		instanceID:              "my-instance",
		passFailStep:            true,
		workDir:                 workDir,
		testName:                "pixel-tests",
		pngFile:                 filepath.Join(td, "00000000000000000000000000000000.png"),
		testKeysStrings:         []string{"os:Android", "gpu:Mali600"},
		testOptionalKeysStrings: []string{"some_option:is_optional"},
	}

	output := bytes.Buffer{}
	exit := &exitCodeRecorder{}
	ctx := executionContext(context.Background(), &output, &output, exit.ExitWithCode)
	ctx = goldclient.WithContext(ctx, nil, nil, nil)

	runUntilExit(t, func() {
		env.Add(ctx)
	})
	exit.AssertWasCalledWithCode(t, 0, output.String())
}
