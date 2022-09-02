package jsonio

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/golden/go/types"
)

func TestValidate_InvalidData_ReturnsError(t *testing.T) {

	test := func(name string, toValidate GoldResults, errFragment string) {
		t.Run(name, func(t *testing.T) {
			err := toValidate.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), errFragment)
		})
	}

	test("empty", GoldResults{}, `"gitHash", "commit_id", or "change_list_id" must be set`)
	test("invalidHash", GoldResults{
		GitHash: "whoops this isn't hexadecimal",
		Key:     map[string]string{"param1": "value1"},
	}, `must be hexadecimal`)
	test("missingKey", GoldResults{
		GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
	}, `field "key" must not be empty`)
	test("emptyKeyResults", GoldResults{
		GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
		Key:     map[string]string{"param1": "value1"},
		Results: []Result{
			{
				Key:    map[string]string{},
				Digest: "12345abc",
			},
		},
	}, `results" index 0: field "key" must not be empty`)
	test("emptyKeyValue", GoldResults{
		GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
		Key:     map[string]string{"param1": ""},
		Results: []Result{
			{
				Key: map[string]string{
					types.PrimaryKeyField: "foo",
					types.CorpusField:     "my corpus",
				},
				Digest: "12345abc",
			},
		},
	}, `field "key" must not have empty keys`)
	test("noNameField", GoldResults{
		GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
		Key:     map[string]string{"param1": "value1"},
		Results: []Result{
			{
				Key: map[string]string{
					"no_name": "bar",
				},
				Digest: "12345abc",
			},
		},
	}, `field "key" is missing key name`)
	test("noCorpusField", GoldResults{
		GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
		Key:     map[string]string{"param1": "value1"},
		Results: []Result{
			{
				Key: map[string]string{
					types.PrimaryKeyField: "bar",
				},
				Digest: "12345abc",
			},
		},
	}, `field "key" is missing key source_type`)
	test("missingDigest", GoldResults{
		GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
		Key:     map[string]string{"param1": "value1"},
		Results: []Result{
			{
				Key: map[string]string{
					types.PrimaryKeyField: "bar",
				},
			},
		},
	}, `missing digest`)
	test("invalidDigest", GoldResults{
		GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
		Key:     map[string]string{"param1": "value1"},
		Results: []Result{
			{
				Key: map[string]string{
					types.PrimaryKeyField: "bar",
				},
				Digest: "not hexadecimal",
			},
		},
	}, `must be hexadecimal`)
	test("missingOptions", GoldResults{
		GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
		Key:     map[string]string{"param1": "value1"},
		Results: []Result{
			{
				Key: map[string]string{
					types.PrimaryKeyField: "bar",
				},
				Digest: "abc123",
				Options: map[string]string{
					"oops": "",
				},
			},
		},
	}, `with key "oops"`)
	test("missingOptions2", GoldResults{
		GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
		Key:     map[string]string{"param1": "value1"},
		Results: []Result{
			{
				Key: map[string]string{
					types.PrimaryKeyField: "bar",
					types.CorpusField:     "my corpus",
				},
				Digest: "abc123",
				Options: map[string]string{
					"": "missing",
				},
			},
		},
	}, `with value "missing"`)
	test("missingOptions3", GoldResults{
		GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
		Key:     map[string]string{"param1": "value1"},
		Results: []Result{
			{
				Key: map[string]string{
					types.PrimaryKeyField: "bar",
					types.CorpusField:     "my corpus",
				},
				Digest: "abc123",
				Options: map[string]string{
					"": "",
				},
			},
		},
	}, `empty key and value`)
	test("partialChangelistInfo", GoldResults{
		GitHash:          "aaa27ef254ad66609606c7af0730ee062b25edf9",
		Key:              map[string]string{"param1": "value1"},
		ChangelistID:     "missing_tryjob",
		CodeReviewSystem: "some_system",
		PatchsetOrder:    1,
	}, `all of or none of`)
	test("partialChangelistInfo2", GoldResults{
		Key:                         map[string]string{"param1": "value1"},
		ChangelistID:                "missing_patchset",
		CodeReviewSystem:            "some_system",
		PatchsetOrder:               0, // order, by definition, starts at 1
		TryJobID:                    "12345",
		ContinuousIntegrationSystem: "sandbucket",
	}, `all of or none of`)
	test("partialChangelistInfo3", GoldResults{
		GitHash:                     "aaa27ef254ad66609606c7af0730ee062b25edf9",
		Key:                         map[string]string{"param1": "value1"},
		TryJobID:                    "12345",
		ContinuousIntegrationSystem: "sandbucket",
	}, `all of or none of`)
	test("partialChangelistInfo4", GoldResults{
		GitHash:                     "aaa27ef254ad66609606c7af0730ee062b25edf9",
		Key:                         map[string]string{"param1": "value1"},
		ChangelistID:                "missing_patchset_id",
		CodeReviewSystem:            "some_system",
		PatchsetID:                  "",
		TryJobID:                    "12345",
		ContinuousIntegrationSystem: "sandbucket",
	}, `all of or none of`)
	test("partialChangelistInfo5", GoldResults{
		Key:                         map[string]string{"param1": "value1"},
		ChangelistID:                "missing_tryjob",
		CodeReviewSystem:            "some_system",
		PatchsetID:                  "12345",
		TryJobID:                    "",
		ContinuousIntegrationSystem: "sandbucket",
	}, `all of or none of`)
}

func TestValidate_ValidResults_Success(t *testing.T) {

	test := func(name string, toValidate GoldResults) {
		t.Run(name, func(t *testing.T) {
			err := toValidate.Validate()
			require.NoError(t, err)
		})
	}

	test("primaryBranch", GoldResults{
		GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
		Key:     map[string]string{"param1": "value1"},
		Results: []Result{
			{
				Key: map[string]string{
					types.PrimaryKeyField: "bar",
					types.CorpusField:     "my corpus",
				},
				Digest: "12345abc",
			},
		},
	})
	test("With CommitID And Metadata", GoldResults{
		CommitID:       "R89-13729.8.0",
		CommitMetadata: "gs://chromeos-image-archive/release/R89-13729.8.0/manifest.xml",
		Key:            map[string]string{"param1": "value1"},
		Results: []Result{
			{
				Key: map[string]string{
					types.PrimaryKeyField: "bar",
					types.CorpusField:     "my corpus",
				},
				Digest: "12345abc",
			},
		},
	})
	test("onChangelist", GoldResults{
		GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
		Key:     map[string]string{"param1": "value1"},
		Results: []Result{
			{
				Key: map[string]string{
					types.PrimaryKeyField: "bar",
					types.CorpusField:     "my corpus",
				},
				Digest: "12345abc",
			},
		},
		ChangelistID:                "123456",
		CodeReviewSystem:            "some_system",
		PatchsetOrder:               1,
		TryJobID:                    "12345",
		ContinuousIntegrationSystem: "sandbucket",
	})
	test("Data for CL without GitHash", GoldResults{
		Key: map[string]string{"param1": "value1"},
		Results: []Result{
			{
				Key: map[string]string{
					types.PrimaryKeyField: "bar",
					types.CorpusField:     "my corpus",
				},
				Digest: "12345abc",
			},
		},
		ChangelistID:                "123456",
		CodeReviewSystem:            "some_system",
		PatchsetOrder:               1,
		TryJobID:                    "12345",
		ContinuousIntegrationSystem: "sandbucket",
	})
	test("withPatchsetID", GoldResults{
		GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
		Key:     map[string]string{"param1": "value1"},
		Results: []Result{
			{
				Key: map[string]string{
					types.PrimaryKeyField: "bar",
					types.CorpusField:     "my corpus",
				},
				Digest: "12345abc",
			},
		},
		ChangelistID:                "123456",
		CodeReviewSystem:            "some_system",
		PatchsetID:                  "another id",
		TryJobID:                    "12345",
		ContinuousIntegrationSystem: "sandbucket",
	})
	test("test name and corpus in key", GoldResults{
		GitHash: "aaa27ef254ad66609606c7af0730ee062b25edf9",
		Key: map[string]string{
			types.CorpusField:     "my corpus",
			types.PrimaryKeyField: "bar",
		},
		Results: []Result{
			{
				Key: map[string]string{
					"param1": "value1",
				},
				Digest: "1234567890abcdef",
			},
		},
	})
}

// TestUpdateLegacyFields_Success tests a variety of valid inputs to make sure our parsing logic
// does not regress. It handles a variety of legacy and non legacy data.
func TestUpdateLegacyFields_Success(t *testing.T) {
	r := parseUpdateValidate(t, legacySkiaTryjobJSON)

	require.Equal(t, "c4711517219f333c1116f47706eb57b51b5f8fc7", r.GitHash)
	require.Equal(t, "12345", r.ChangelistID)
	require.Equal(t, 10, r.PatchsetOrder)
	require.Equal(t, "549340494940393", r.TryJobID)
	// When we detect a legacy system, default to gerrit and buildbucket
	require.Equal(t, "gerrit", r.CodeReviewSystem)
	require.Equal(t, "buildbucket", r.ContinuousIntegrationSystem)
	require.Len(t, r.Results, 3)

	r = parseUpdateValidate(t, legacySkiaJSON)
	require.Empty(t, r.ChangelistID)
	require.Empty(t, r.TryJobID)
	require.Equal(t, r.Results[0].Key[types.PrimaryKeyField], "skottie_multiframe")
	require.Contains(t, r.Results[0].Options, "color_type")

	r = parseUpdateValidate(t, legacyGoldCtlTryjobJSON)
	require.Equal(t, "1762193", r.ChangelistID)
	require.Equal(t, 2, r.PatchsetOrder)
	require.Equal(t, "8904604368086838672", r.TryJobID)
	// When we detect a legacy system, default to gerrit and buildbucket
	require.Equal(t, "gerrit", r.CodeReviewSystem)
	require.Equal(t, "buildbucket", r.ContinuousIntegrationSystem)
	require.Contains(t, r.Key, "vendor_id")

	r = parseUpdateValidate(t, goldCtlTryjobJSON)
	require.Equal(t, "1762193", r.ChangelistID)
	require.Equal(t, 2, r.PatchsetOrder)
	require.Equal(t, "8904604368086838672", r.TryJobID)
	require.Equal(t, "gerrit", r.CodeReviewSystem)
	require.Equal(t, "buildbucket", r.ContinuousIntegrationSystem)
	require.Contains(t, r.Key, "vendor_id")

	r = parseUpdateValidate(t, goldCtlTryjobPSIDJSON)
	require.Equal(t, "1762193", r.ChangelistID)
	require.Equal(t, "42191ad7b6f31d823d2d9904df24c0649ca3766c", r.PatchsetID)

	r = parseUpdateValidate(t, goldCtlMasterBranchJSON)
	require.Empty(t, r.ChangelistID)
	require.Empty(t, r.TryJobID)
	require.Equal(t, map[string]string{
		"device_id": "0x1cb3",
		"msaa":      "True",
	}, r.Key)

	r = parseUpdateValidate(t, legacyGoldCtlJSON)
	require.Empty(t, r.ChangelistID)
	require.Empty(t, r.TryJobID)
	require.Contains(t, r.Key, "vendor_id")

	r = parseUpdateValidate(t, legacyMasterBranchJSON)
	require.Empty(t, r.ChangelistID)
	require.Empty(t, r.TryJobID)

	r = parseUpdateValidate(t, negativeMasterBranchJSON)
	require.Empty(t, r.ChangelistID)
	require.Empty(t, r.TryJobID)

	r = parseUpdateValidate(t, emptyIssueJSON)
	require.Empty(t, r.ChangelistID)
	require.Empty(t, r.TryJobID)

	parseUpdateValidate(t, inverted_results)
}

func TestUpdateLegacyFields_LexicographicalOrderOfCommitIDsFixed(t *testing.T) {

	test := func(oldID, fixedID string) {
		t.Run(oldID, func(t *testing.T) {
			g := GoldResults{CommitID: oldID}
			require.NoError(t, g.UpdateLegacyFields())
			assert.Equal(t, g.CommitID, fixedID)
		})
	}
	test("R99-14469.8.1", "R099-14469.8.1")
	test("R23-14469.8.1", "R023-14469.8.1")

	// No changes
	test("R100-14470.0.0", "R100-14470.0.0")
	test("R987-14470.0.0", "R987-14470.0.0")
	test("R012-14470.0.0", "R012-14470.0.0")
}

func TestGenJson(t *testing.T) {

	// Test parsing the test JSON.
	goldResults := parseUpdateValidate(t, legacySkiaTryjobJSON)

	// For good measure we validate.
	err := goldResults.Validate()
	require.NoError(t, err)

	// Encode and decode the results.
	var buf bytes.Buffer
	require.NoError(t, json.NewEncoder(&buf).Encode(goldResults))
	newGoldResults := parseUpdateValidate(t, buf.String())
	require.Equal(t, goldResults, newGoldResults)
}

func parseUpdateValidate(t *testing.T, jsonStr string) *GoldResults {
	buf := bytes.NewBuffer([]byte(jsonStr))
	gr := &GoldResults{}
	require.NoError(t, json.NewDecoder(buf).Decode(gr))
	require.NoError(t, gr.UpdateLegacyFields())
	require.NoError(t, gr.Validate())
	return gr
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
		"results": [{
          "key":{"name": "abc", "source_type":"def"},
          "md5": "0123456789abcdef0123456789abcdef"
        }],
		"issue": "0"
	}`

	negativeMasterBranchJSON = `{
		"gitHash" : "c4711517219f333c1116f47706eb57b51b5f8fc7",
		"key" : {
			"arch" : "arm64"
		},
        "results": [{
          "key":{"name": "abc", "source_type":"def"},
          "md5": "0123456789abcdef0123456789abcdef"
        }],
		"issue": "-1"
	}`

	emptyIssueJSON = `{
		"gitHash" : "c4711517219f333c1116f47706eb57b51b5f8fc7",
		"key" : {
			"arch" : "arm64"
		},
		"issue": ""
	}`

	// This is what goldctl should spit out when running on the primary branch
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

	// This is what goldctl could spit out for a tryjob run when specifying patchset id
	// starting after Nov 2019.
	goldCtlTryjobPSIDJSON = `{
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
  "patch_set_id": "42191ad7b6f31d823d2d9904df24c0649ca3766c",
  "crs": "gerrit",
  "try_job_id": "8904604368086838672",
  "cis": "buildbucket"
}`

	inverted_results = `{
  "description": "This file has data from two traces from the same test. It should re-use commit with id 0000000103",
  "gitHash" : "efbb705ff27b6a1a7e8ee36d219a7a0c41225771",
  "key" : {
    "name" : "square",
    "source_type" : "corners"
  },
  "results" : [
    {
      "key" : {
        "os" : "Windows10.3"
      },
      "md5" : "a02a02a02a02a02a02a02a02a02a02a0",
      "options" : {
        "ext" : "png"
      }
    },
    {
      "key" : {
        "os": "Android"
      },
      "md5" : "a02a02a02a02a02a02a02a02a02a02a0",
      "options" : {
        "ext" : "png",
        "image_matching_algorithm": "fuzzy",
        "fuzzy_max_different_pixels": "10",
        "fuzzy_pixel_delta_threshold": "20",
        "fuzzy_ignored_border_thickness": "0"
      }
    }
  ]
}`

	// This is what goldctl spits out before Aug 2019 when run on the waterfall, it
	// should continue to be valid if the schema changes (so we can re-ingest).
	legacyGoldCtlJSON = `{"gitHash":"7d3833876fb941a69bc3f49736eb8912c44156a8","key":{"device_id":"None","device_string":"Adreno (TM) 418","model_name":"Nexus 5X","msaa":"True","vendor_id":"None","vendor_string":"Qualcomm"},"results":[{"key":{"name":"Pixel_CanvasDisplayLinearRGBAccelerated2D","source_type":"chrome-gpu"},"options":{"ext":"png"},"md5":"301c213d21e2e0c8b0a0ddf5771453d2"}],"issue":"0","buildbucket_build_id":"0","patchset":"0","builder":"","task_id":""}`
)
