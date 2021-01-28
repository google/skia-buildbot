package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestDump_AfterInit_Success(t *testing.T) {
	unittest.MediumTest(t)

	workDir := t.TempDir()
	setupAuthWithGSUtil(t, workDir)

	mh := mockRPCResponses().Positive("pixel-tests", blankDigest).
		Negative("other-test", blankDigest).
		Known("11111111111111111111111111111111").Build()

	// Call imgtest init with the following flags. We expect it to load the baseline expectations
	// and the known hashes (both empty).
	ctx, output, exit := testContext(nil, mh, nil, nil)
	initEnv := imgTest{
		commitHash:      "1234567890123456789012345678901234567890",
		corpus:          "my_corpus",
		instanceID:      "my-instance",
		testKeysStrings: []string{"os:Android", "device:angler"},
		workDir:         workDir,
	}
	runUntilExit(t, func() {
		initEnv.Init(ctx)
	})
	exit.AssertWasCalledWithCode(t, 0, output.String())

	ctx, output, exit = testContext(nil, nil, nil, nil)
	eumpEnv := dumpEnv{
		flagDumpHashes:   true,
		flagDumpBaseline: true,
		flagWorkDir:      workDir,
	}
	runUntilExit(t, func() {
		eumpEnv.Dump(ctx)
	})
	logs := output.String()
	exit.AssertWasCalledWithCode(t, 0, logs)

	assert.Equal(t, `Baseline:
other-test:
	00000000000000000000000000000000 : negative
pixel-tests:
	00000000000000000000000000000000 : positive

Known Hashes:
Hashes:
	00000000000000000000000000000000
	11111111111111111111111111111111

`, logs)
}
