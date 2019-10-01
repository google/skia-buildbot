package jsonio

import (
	"bytes"
	"encoding/json"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/types"
)

// TestValidateInvalid tests a bunch of cases that fail validation
func TestValidateInvalid(t *testing.T) {
	unittest.SmallTest(t)

	type invalidInput struct {
		results       GoldResults
		ignoreResults bool
		errFragment   string // should be unique enough to make sure we stopped on the right error.
	}

	tests := map[string]invalidInput{
		"empty": {
			results:       GoldResults{},
			ignoreResults: true,
			errFragment:   `"gitHash" must be hexadecimal`,
		},
		"invalidHash": {
			results: GoldResults{
				GitHash: "whoops this isn't hexadecimal",
				Key:     map[string]string{"param1": "value1"},
			},
			ignoreResults: true,
			errFragment:   `must be hexadecimal`,
		},
		"missingKey": {
			results: GoldResults{
				GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
			},
			ignoreResults: true,
			errFragment:   `field "key" must not be empty`,
		},
		"missingResults": {
			results: GoldResults{
				GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
				Key:     map[string]string{"param1": "value1"},
			},
			ignoreResults: false,
			errFragment:   `field "results" must not be empty`,
		},
		"emptyResults": {
			results: GoldResults{
				GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
				Key:     map[string]string{"param1": "value1"},
				Results: []*Result{},
			},
			ignoreResults: false,
			errFragment:   `field "results" must not be empty`,
		},
		"emptyKeyResults": {
			results: GoldResults{
				GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
				Key:     map[string]string{"param1": "value1"},
				Results: []*Result{
					{
						Key:    map[string]string{},
						Digest: "12345abc",
					},
				},
			},
			ignoreResults: false,
			errFragment:   `results" index 0: field "key" must not be empty`,
		},
		// TODO(kjlubick) chrome-gpu is currently uploading a key with an empty value
		//   Re-enable this check once they are fixed.
		//"emptyKeyValue": {
		//	results: GoldResults{
		//		GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
		//		Key:     map[string]string{"param1": ""},
		//		Results: []*Result{
		//			{
		//				Key: map[string]string{
		//					types.PRIMARY_KEY_FIELD: "foo",
		//				},
		//				Digest: "12345abc",
		//			},
		//		},
		//	},
		//	ignoreResults: false,
		//	errFragment:   `field "key" must not have empty keys`,
		//},
		"noNameField": {
			results: GoldResults{
				GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
				Key:     map[string]string{"param1": "value1"},
				Results: []*Result{
					{
						Key: map[string]string{
							"no_name": "bar",
						},
						Digest: "12345abc",
					},
				},
			},
			ignoreResults: false,
			errFragment:   `field "key" is missing key name`,
		},
		"missingDigest": {
			results: GoldResults{
				GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
				Key:     map[string]string{"param1": "value1"},
				Results: []*Result{
					{
						Key: map[string]string{
							types.PRIMARY_KEY_FIELD: "bar",
						},
					},
				},
			},
			ignoreResults: false,
			errFragment:   `missing digest`,
		},
		"invalidDigest": {
			results: GoldResults{
				GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
				Key:     map[string]string{"param1": "value1"},
				Results: []*Result{
					{
						Key: map[string]string{
							types.PRIMARY_KEY_FIELD: "bar",
						},
						Digest: "not hexadecimal",
					},
				},
			},
			ignoreResults: false,
			errFragment:   `must be hexadecimal`,
		},
		"missingOptions": {
			results: GoldResults{
				GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
				Key:     map[string]string{"param1": "value1"},
				Results: []*Result{
					{
						Key: map[string]string{
							types.PRIMARY_KEY_FIELD: "bar",
						},
						Digest: "abc123",
						Options: map[string]string{
							"oops": "",
						},
					},
				},
			},
			ignoreResults: false,
			errFragment:   `with key "oops"`,
		},
		"missingOptions2": {
			results: GoldResults{
				GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
				Key:     map[string]string{"param1": "value1"},
				Results: []*Result{
					{
						Key: map[string]string{
							types.PRIMARY_KEY_FIELD: "bar",
						},
						Digest: "abc123",
						Options: map[string]string{
							"": "missing",
						},
					},
				},
			},
			ignoreResults: false,
			errFragment:   `with value "missing"`,
		},
		"missingOptions3": {
			results: GoldResults{
				GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
				Key:     map[string]string{"param1": "value1"},
				Results: []*Result{
					{
						Key: map[string]string{
							types.PRIMARY_KEY_FIELD: "bar",
						},
						Digest: "abc123",
						Options: map[string]string{
							"": "",
						},
					},
				},
			},
			ignoreResults: false,
			errFragment:   `empty key and value`,
		},
		"partialChangeListInfo": {
			results: GoldResults{
				GitHash:          "aaa27ef254ad66609606c7af0730ee062b25edf9",
				Key:              map[string]string{"param1": "value1"},
				ChangeListID:     "missing_tryjob",
				CodeReviewSystem: "some_system",
				PatchSetOrder:    1,
			},
			ignoreResults: true,
			errFragment:   `all of or none of`,
		},
		"partialChangeListInfo2": {
			results: GoldResults{
				GitHash:                     "aaa27ef254ad66609606c7af0730ee062b25edf9",
				Key:                         map[string]string{"param1": "value1"},
				ChangeListID:                "missing_patchset",
				CodeReviewSystem:            "some_system",
				PatchSetOrder:               0, // order, by definition, starts at 1
				TryJobID:                    "12345",
				ContinuousIntegrationSystem: "sandbucket",
			},
			ignoreResults: true,
			errFragment:   `all of or none of`,
		},
		"partialChangeListInfo3": {
			results: GoldResults{
				GitHash:                     "aaa27ef254ad66609606c7af0730ee062b25edf9",
				Key:                         map[string]string{"param1": "value1"},
				TryJobID:                    "12345",
				ContinuousIntegrationSystem: "sandbucket",
			},
			ignoreResults: true,
			errFragment:   `all of or none of`,
		},
	}

	for name, testCase := range tests {
		err := testCase.results.Validate(testCase.ignoreResults)
		assert.Error(t, err, "when processing %s: %v", name, testCase)
		assert.Contains(t, err.Error(), testCase.errFragment, name)
		assert.NotEmpty(t, testCase.errFragment, "write an assertion for %s - %s", name, err.Error())
	}
}

// TestValidateValid tests a few cases that pass validation
func TestValidateValid(t *testing.T) {
	unittest.SmallTest(t)

	type validInput struct {
		results       GoldResults
		ignoreResults bool
	}

	tests := map[string]validInput{
		"emptyResultsButResultsIgnored": {
			results: GoldResults{
				GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
				Key:     map[string]string{"param1": "value1"},
				Results: []*Result{},
			},
			ignoreResults: true,
		},
		"masterBranch": {
			results: GoldResults{
				GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
				Key:     map[string]string{"param1": "value1"},
				Results: []*Result{
					{
						Key: map[string]string{
							types.PRIMARY_KEY_FIELD: "bar",
						},
						Digest: "12345abc",
					},
				},
			},
			ignoreResults: false,
		},
		"onChangeList": {
			results: GoldResults{
				GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
				Key:     map[string]string{"param1": "value1"},
				Results: []*Result{
					{
						Key: map[string]string{
							types.PRIMARY_KEY_FIELD: "bar",
						},
						Digest: "12345abc",
					},
				},
				ChangeListID:                "123456",
				CodeReviewSystem:            "some_system",
				PatchSetOrder:               1,
				TryJobID:                    "12345",
				ContinuousIntegrationSystem: "sandbucket",
			},
			ignoreResults: false,
		},
	}

	for name, testCase := range tests {
		err := testCase.results.Validate(testCase.ignoreResults)
		assert.NoError(t, err, "when processing %s: %v", name, testCase)
	}

}

// TestParseGoldResultsValid tests a variety of valid inputs to make sure our parsing logic
// does not regress. It handles a variety of legacy and non legacy data.
func TestParseGoldResultsValid(t *testing.T) {
	unittest.SmallTest(t)
	r := testParse(t, legacySkiaTryjobJSON)

	// Make sure some key fields come out correctly, i.e. are converted correctly from string to int.
	assert.Equal(t, "c4711517219f333c1116f47706eb57b51b5f8fc7", r.GitHash)
	assert.Equal(t, "Xb0VhENPSRFGnf2elVQd", r.TaskID)
	assert.Equal(t, "12345", r.ChangeListID)
	assert.Equal(t, 10, r.PatchSetOrder)
	assert.Equal(t, "549340494940393", r.TryJobID)
	// When we detect a legacy system, default to gerrit and buildbucket
	assert.Equal(t, "gerrit", r.CodeReviewSystem)
	assert.Equal(t, "buildbucket", r.ContinuousIntegrationSystem)
	assert.Len(t, r.Results, 3)

	r = testParse(t, legacySkiaJSON)
	assert.Empty(t, r.ChangeListID)
	assert.Empty(t, r.TryJobID)
	assert.Equal(t, "Test-Android-Clang-Nexus7-CPU-Tegra3-arm-Release-All-Android", r.Builder)
	assert.Equal(t, r.Results[0].Key[types.PRIMARY_KEY_FIELD], "skottie_multiframe")
	assert.Contains(t, r.Results[0].Options, "color_type")

	r = testParse(t, legacyGoldCtlTryjobJSON)
	assert.Equal(t, "1762193", r.ChangeListID)
	assert.Equal(t, 2, r.PatchSetOrder)
	assert.Equal(t, "8904604368086838672", r.TryJobID)
	// When we detect a legacy system, default to gerrit and buildbucket
	assert.Equal(t, "gerrit", r.CodeReviewSystem)
	assert.Equal(t, "buildbucket", r.ContinuousIntegrationSystem)
	assert.Contains(t, r.Key, "vendor_id")

	r = testParse(t, goldCtlTryjobJSON)
	assert.Equal(t, "1762193", r.ChangeListID)
	assert.Equal(t, 2, r.PatchSetOrder)
	assert.Equal(t, "8904604368086838672", r.TryJobID)
	assert.Equal(t, "gerrit", r.CodeReviewSystem)
	assert.Equal(t, "buildbucket", r.ContinuousIntegrationSystem)
	assert.Contains(t, r.Key, "vendor_id")

	r = testParse(t, goldCtlMasterBranchJSON)
	assert.Empty(t, r.ChangeListID)
	assert.Empty(t, r.TryJobID)
	assert.Equal(t, map[string]string{
		"device_id": "0x1cb3",
		"msaa":      "True",
	}, r.Key)

	r = testParse(t, legacyGoldCtlJSON)
	assert.Empty(t, r.ChangeListID)
	assert.Empty(t, r.TryJobID)
	assert.Contains(t, r.Key, "vendor_id")

	r = testParse(t, legacyMasterBranchJSON)
	assert.Empty(t, r.ChangeListID)
	assert.Empty(t, r.TryJobID)

	r = testParse(t, negativeMasterBranchJSON)
	assert.Empty(t, r.ChangeListID)
	assert.Empty(t, r.TryJobID)

	r = testParse(t, emptyIssueJSON)
	assert.Empty(t, r.ChangeListID)
	assert.Empty(t, r.TryJobID)
}

func TestGenJson(t *testing.T) {
	unittest.SmallTest(t)

	// Test parsing the test JSON.
	goldResults := testParse(t, legacySkiaTryjobJSON)

	// For good measure we validate.
	err := goldResults.Validate(false)
	assert.NoError(t, err)

	// Encode and decode the results.
	var buf bytes.Buffer
	assert.NoError(t, json.NewEncoder(&buf).Encode(goldResults))
	newGoldResults := testParse(t, buf.String())
	assert.Equal(t, goldResults, newGoldResults)
}

func testParse(t *testing.T, jsonStr string) *GoldResults {
	buf := bytes.NewBuffer([]byte(jsonStr))

	ret, err := ParseGoldResults(buf)
	assert.NoError(t, err)
	return ret
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

	negativeMasterBranchJSON = `{
		"gitHash" : "c4711517219f333c1116f47706eb57b51b5f8fc7",
		"key" : {
			"arch" : "arm64"
		},
		"issue": "-1"
	}`

	emptyIssueJSON = `{
		"gitHash" : "c4711517219f333c1116f47706eb57b51b5f8fc7",
		"key" : {
			"arch" : "arm64"
		},
		"issue": ""
	}`

	// This is what goldctl should spit out when running on the master branch
	goldCtlMasterBranchJSON = `{
  "gitHash": "e1681c90cf6a4c3b6be2bc4b4cea59849c16a438",
  "key": {
    "device_id": "0x1cb3",
    "msaa": "True"
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
  ]
}`

	// This is what goldctl spits out before Aug 2019 when run on a tryjob, it
	// should continue to be valid if the schema changes (so we can re-ingest).
	legacyGoldCtlTryjobJSON = `{
  "gitHash": "e1681c90cf6a4c3b6be2bc4b4cea59849c16a438",
  "key": {
    "device_id": "0x1cb3",
    "device_string": "None",
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

	// This is what goldctl should spit out for a tryjob run starting after Sept 2019.
	goldCtlTryjobJSON = `{
  "gitHash": "e1681c90cf6a4c3b6be2bc4b4cea59849c16a438",
  "key": {
    "device_id": "0x1cb3",
    "device_string": "None",
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
  "change_list_id": "1762193",
  "patch_set_order": 2,
  "crs": "gerrit",
  "try_job_id": "8904604368086838672",
  "cis": "buildbucket"
}`

	// This is what goldctl spits out before Aug 2019 when run on the waterfall, it
	// should continue to be valid if the schema changes (so we can re-ingest).
	legacyGoldCtlJSON = `{"gitHash":"7d3833876fb941a69bc3f49736eb8912c44156a8","key":{"device_id":"None","device_string":"Adreno (TM) 418","model_name":"Nexus 5X","msaa":"True","vendor_id":"None","vendor_string":"Qualcomm"},"results":[{"key":{"name":"Pixel_CanvasDisplayLinearRGBAccelerated2D","source_type":"chrome-gpu"},"options":{"ext":"png"},"md5":"301c213d21e2e0c8b0a0ddf5771453d2"}],"issue":"0","buildbucket_build_id":"0","patchset":"0","builder":"","task_id":""}`
)
