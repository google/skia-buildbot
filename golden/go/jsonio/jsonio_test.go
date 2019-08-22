package jsonio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/types"
)

func TestValidate(t *testing.T) {
	unittest.SmallTest(t)

	empty := &GoldResults{}
	errMsgs, err := empty.Validate(false)
	assert.Error(t, err)
	assertErrorFields(t, errMsgs,
		"gitHash",
		"key",
		"results")
	assert.NotNil(t, errMsgs)

	wrongResults := &GoldResults{
		GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
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
		{Key: map[string]string{}},
	}
	errMsgs, err = wrongResults.Validate(false)
	assert.Error(t, err)
	assertErrorFields(t, errMsgs, "results")

	// Now ignore the results in the validation.
	errMsgs, err = wrongResults.Validate(true)
	assert.NoError(t, err)
	assert.Equal(t, []string(nil), errMsgs)

	// Check that the Validate accounts for both MasterBranch and
	// LegacyMasterBranch values.
	legacyMaster := &GoldResults{
		GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
		Key:     map[string]string{"param1": "value1"},

		GerritChangeListID: types.LegacyMasterBranch,
	}
	_, err = legacyMaster.Validate(true)
	assert.NoError(t, err)

	master := &GoldResults{
		GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
		Key:     map[string]string{"param1": "value1"},

		GerritChangeListID: types.MasterBranch,
	}
	_, err = master.Validate(true)
	assert.NoError(t, err)
}

func TestParseGoldResults(t *testing.T) {
	unittest.SmallTest(t)
	r := testParse(t, legacySkiaTryjobJSON)

	// Make sure some key fields come out correctly, i.e. are converted correctly from string to int.
	assert.Equal(t, "c4711517219f333c1116f47706eb57b51b5f8fc7", r.GitHash)
	assert.Equal(t, "Xb0VhENPSRFGnf2elVQd", r.TaskID)
	assert.Equal(t, int64(12345), r.GerritChangeListID)
	assert.Equal(t, int64(10), r.GerritPatchSet)
	assert.Equal(t, int64(549340494940393), r.BuildBucketID)
	assert.Len(t, r.Results, 3)

	r = testParse(t, legacySkiaJSON)
	assert.Equal(t, types.MasterBranch, r.GerritChangeListID)
	assert.Equal(t, "Test-Android-Clang-Nexus7-CPU-Tegra3-arm-Release-All-Android", r.Builder)
	assert.Equal(t, r.Results[0].Key[types.PRIMARY_KEY_FIELD], "skottie_multiframe")
	assert.Contains(t, r.Results[0].Options, "color_type")

	r = testParse(t, legacyGoldCtlTryjobJSON)
	assert.Equal(t, int64(1762193), r.GerritChangeListID)
	assert.Equal(t, int64(2), r.GerritPatchSet)
	assert.Equal(t, int64(8904604368086838672), r.BuildBucketID)
	assert.Contains(t, r.Key, "vendor_id")

	r = testParse(t, legacyGoldCtlJSON)
	assert.Equal(t, types.LegacyMasterBranch, r.GerritChangeListID)
	assert.Contains(t, r.Key, "vendor_id")

	r = testParse(t, legacyMasterBranchJSON)
	assert.Equal(t, types.LegacyMasterBranch, r.GerritChangeListID)

	r = testParse(t, masterBranchJSON)
	assert.Equal(t, types.MasterBranch, r.GerritChangeListID)

	r = testParse(t, emptyMasterBranchJSON)
	assert.Equal(t, types.MasterBranch, r.GerritChangeListID)
}

func TestGenJson(t *testing.T) {
	unittest.SmallTest(t)

	// Test parsing the test JSON.
	goldResults := testParse(t, legacySkiaTryjobJSON)

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

const (
	// This is what Skia uploads before Aug 2019 when run on a tryjob, it
	// should continue to be valid if the schema changes (so we can re-ingest).
	legacySkiaTryjobJSON = `{
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
		"task_id" : "Xb0VhENPSRFGnf2elVQd",
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

	legacySkiaJSON = `{
   "gitHash": "9c23a9e790b2f29b2cf204e67dbc67a363d0ce74",
   "builder": "Test-Android-Clang-Nexus7-CPU-Tegra3-arm-Release-All-Android",
   "buildbucket_build_id": "0",
   "task_id": "56CzL86rgSDVSkFVK28r",
   "swarming_bot_id": "skia-rpi-012",
   "swarming_task_id": "469891c521e43d11",
   "key": {
      "arch": "arm",
      "compiler": "Clang",
      "extra_config": "Android",
      "model": "Nexus7",
      "os": "Android",
      "style": "default"
   },
   "max_rss_MB": 274,
   "results": [
      {
         "key": {
            "name": "skottie_multiframe",
            "config": "8888",
            "source_type": "gm"
         },
         "options": {
            "ext": "png",
            "gamut": "untagged",
            "transfer_fn": "untagged",
            "color_type": "RGBA_8888",
            "alpha_type": "Premul",
            "color_depth": "8888"
         },
         "md5": "0abe3a2f7f58d2943f3b8b87f91dbff0"
      },
      {
         "key": {
            "name": "HTC.dng",
            "config": "8888",
            "source_type": "colorImage",
            "source_options": "decode_native"
         },
         "options": {
            "ext": "png",
            "gamut": "untagged",
            "transfer_fn": "untagged",
            "color_type": "RGBA_8888",
            "alpha_type": "Premul",
            "color_depth": "8888"
         },
         "md5": "9cb31c854f22413841354f98b22a9acc"
      },
      {
         "key": {
            "name": "HTC.dng",
            "config": "8888",
            "source_type": "colorImage",
            "source_options": "decode_to_dst"
         },
         "options": {
            "ext": "png",
            "gamut": "untagged",
            "transfer_fn": "untagged",
            "color_type": "RGBA_8888",
            "alpha_type": "Premul",
            "color_depth": "8888"
         },
         "md5": "9cb31c854f22413841354f98b22a9acc"
      }
   ]
}`

	legacyMasterBranchJSON = `{
		"gitHash" : "c4711517219f333c1116f47706eb57b51b5f8fc7",
		"key" : {
			"arch" : "arm64"
		},
		"issue": "0"
	}`

	masterBranchJSON = `{
		"gitHash" : "c4711517219f333c1116f47706eb57b51b5f8fc7",
		"key" : {
			"arch" : "arm64"
		},
		"issue": "-1"
	}`

	emptyMasterBranchJSON = `{
		"gitHash" : "c4711517219f333c1116f47706eb57b51b5f8fc7",
		"key" : {
			"arch" : "arm64"
		},
		"issue": ""
	}`

	// This is what goldctl spits out before Aug 2019 when run on a tryjob, it
	// should continue to be valid if the schema changes (so we can re-ingest).
	legacyGoldCtlTryjobJSON = `{
  "gitHash": "e1681c90cf6a4c3b6be2bc4b4cea59849c16a438",
  "key": {
    "device_id": "0x1cb3",
    "device_string": "None",
    "model_name": "",
    "msaa": "True",
    "vendor_id": "0x10de",
    "vendor_string": "None"
  },
  "results": [
    {
      "key": {
        "name": "Pixel_CanvasDisplayLinearRGBUnaccelerated2DGPUCompositing",
        "source_type": "chrome-gpu"
      },
      "options": {
        "ext": "png"
      },
      "md5": "690f72c0b56ae014c8ac66e7f25c0779"
    }
  ],
  "issue": "1762193",
  "buildbucket_build_id": "8904604368086838672",
  "patchset": "2",
  "builder": "",
  "task_id": ""
}`

	// This is what goldctl spits out before Aug 2019 when run on the waterfall, it
	// should continue to be valid if the schema changes (so we can re-ingest).
	legacyGoldCtlJSON = `{"gitHash":"7d3833876fb941a69bc3f49736eb8912c44156a8","key":{"device_id":"None","device_string":"Adreno (TM) 418","model_name":"Nexus 5X","msaa":"True","vendor_id":"None","vendor_string":"Qualcomm"},"results":[{"key":{"name":"Pixel_CanvasDisplayLinearRGBAccelerated2D","source_type":"chrome-gpu"},"options":{"ext":"png"},"md5":"301c213d21e2e0c8b0a0ddf5771453d2"}],"issue":"0","buildbucket_build_id":"0","patchset":"0","builder":"","task_id":""}`
)
