package jsonio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

var (
	testJSON = `
	{
		"gitHash" : "c4711517219f333c1116f47706eb57b51b5f8fc7",
		"key" : {
			 "arch" : "arm64",
			 "compiler" : "Clang",
			 "configuration" : "Debug",
			 "cpu_or_gpu" : "GPU",
			 "cpu_or_gpu_value" : "PowerVRGT7600",
			 "extra_config" : "Metal",
			 "model" : "iPhone7",
			 "os" : "iOS"
		},
		"issue": "12345",
		"patchset": "10",
		"buildbucket_build_id" : "549340494940393",
		"builder" : "Test-Android-Clang-iPhone7-GPU-PowerVRGT7600-arm64-Debug-All-Metal",
		"swarming_bot_id" : "skia-rpi-102",
		"swarming_task_id" : "3fcd8d4a539ba311",
		"taskID" : "Xb0VhENPSRFGnf2elVQd",
		"results" : [
			 {
					"key" : {
						 "config" : "mtl",
						 "name" : "yuv_nv12_to_rgb_effect",
						 "source_type" : "gm"
					},
					"md5" : "30a470b6ac174aa1ffb54fcb77a21f21",
					"options" : {
						 "ext" : "png",
						 "gamma_correct" : "no"
					}
			 },
			 {
					"key" : {
						 "config" : "mtl",
						 "name" : "yuv_to_rgb_effect",
						 "source_type" : "gm"
					},
					"md5" : "0ea32027e1e651e4250797aa44bfadaa",
					"options" : {
						 "ext" : "png",
						 "gamma_correct" : "no"
					}
			 },
			 {
					"key" : {
						 "config" : "pipe-8888",
						 "name" : "clipcubic",
						 "source_type" : "gm"
					},
					"md5" : "64e446d96bebba035887dd7dda6db6c4",
					"options" : {
						 "ext" : "png"
					}
			 }
		]
 }`
)

func TestValidate(t *testing.T) {
	testutils.SmallTest(t)

	empty := &GoldResults{}
	errMsgs, err := empty.Validate(false)
	assert.Error(t, err)
	assertErrorFields(t, errMsgs,
		"gitHash",
		"key",
		"results")
	assert.NotNil(t, errMsgs)

	wrongResults := &GoldResults{
		GitHash: "a1b2c3d4e5f6a7b8c9d0e1f2",
		Key:     map[string]string{"param1": "value1"},
	}
	errMsgs, err = wrongResults.Validate(false)
	assert.Error(t, err)
	assertErrorFields(t, errMsgs, "results")

	wrongResults.Results = []*Result{}
	errMsgs, err = wrongResults.Validate(false)
	assert.Error(t, err)
	assertErrorFields(t, errMsgs, "results")

	wrongResults.Results = []*Result{
		&Result{Key: map[string]string{}},
	}
	errMsgs, err = wrongResults.Validate(false)
	assert.Error(t, err)
	assertErrorFields(t, errMsgs, "results")

	// Now ignore the results in the validation.
	errMsgs, err = wrongResults.Validate(true)
	assert.NoError(t, err)
	assert.Equal(t, []string(nil), errMsgs)
}

func TestParseGoldResults(t *testing.T) {
	testutils.SmallTest(t)
	r := testParse(t, testJSON)

	// Make sure some key fields come out correctly, i.e. are converted correctly from string to int.
	assert.Equal(t, "c4711517219f333c1116f47706eb57b51b5f8fc7", r.GitHash)
	assert.Equal(t, "Xb0VhENPSRFGnf2elVQd", r.TaskID)
	assert.Equal(t, int64(12345), r.Issue)
	assert.Equal(t, int64(10), r.Patchset)
	assert.Equal(t, int64(549340494940393), r.BuildBucketID)
}

func TestGenJson(t *testing.T) {
	testutils.SmallTest(t)

	// Test parsing the test JSON.
	goldResults := testParse(t, testJSON)

	// For good measure we validate.
	_, err := goldResults.Validate(false)
	assert.NoError(t, err)

	// Encode and decode the results.
	var buf bytes.Buffer
	assert.NoError(t, json.NewEncoder(&buf).Encode(goldResults))
	newGoldResults := testParse(t, buf.String())
	assert.Equal(t, goldResults, newGoldResults)
}

func testParse(t *testing.T, jsonStr string) *GoldResults {
	buf := bytes.NewBuffer([]byte(jsonStr))

	ret, errMsg, err := ParseGoldResults(buf)
	assert.NoError(t, err)
	assert.Nil(t, errMsg)
	return ret
}

func assertErrorFields(t *testing.T, errMsgs []string, expectedFields ...string) {
	for _, msg := range errMsgs {
		found := false
		for _, ef := range expectedFields {
			found = found || strings.Contains(msg, ef)
		}
		assert.True(t, found, fmt.Sprintf("Could not find %v in msg: %s", expectedFields, msg))
	}
}
