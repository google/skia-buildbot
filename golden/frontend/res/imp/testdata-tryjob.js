"use strict"

var testdata = testdata || {};

testdata.trybotResults = {
  "NumMatches": 0,
  "commits": null,
  "digests": [
    {
      "blame": null,
      "closestRef": "pos",
      "digest": "68fbc3eed92c40b835b477ae578ee82f",
      "paramset": {
        "debug": [
          "true",
          "false"
        ],
        "ext": [
          "png"
        ],
        "gamma_correct": [
          "no"
        ],
        "is_standalone": [
          "true"
        ],
        "jumbo_file_merge_limit": [
          "50"
        ],
        "name": [
          "Test_CheckBox.pdf.0"
        ],
        "os": [
          "linux"
        ],
        "skia": [
          "false"
        ],
        "source_type": [
          "pdfium"
        ],
        "use_jumbo_build": [
          "true"
        ],
        "v8": [
          "true"
        ],
        "xfa": [
          "true"
        ]
      },
      "refDiffs": {
        "neg": null,
        "pos": {
          "diffs": {
            "combined": 0.05257053,
            "percent": 0.0049514757,
            "pixel": 24
          },
          "digest": "fdbe3470f6ef6627e3d40570c09c1a82",
          "dimDiffer": false,
          "maxRGBADiffs": [
            164,
            167,
            162,
            0
          ],
          "n": 86,
          "numDiffPixels": 24,
          "paramset": {
            "debug": [
              "true",
              "false"
            ],
            "ext": [
              "png"
            ],
            "gamma_correct": [
              "no"
            ],
            "is_standalone": [
              "true"
            ],
            "name": [
              "Test_CheckBox.pdf.0"
            ],
            "os": [
              "mac"
            ],
            "skia": [
              "false"
            ],
            "source_type": [
              "pdfium"
            ],
            "v8": [
              "true"
            ],
            "xfa": [
              "true"
            ]
          },
          "pixelDiffPercent": 0.0049514757,
          "status": "positive",
          "test": ""
        }
      },
      "status": "untriaged",
      "test": "Test_CheckBox.pdf.0",
      "traces": {
        "digests": [
          {
            "digest": "68fbc3eed92c40b835b477ae578ee82f",
            "status": "untriaged"
          }
        ],
        "tileSize": 50,
        "traces": []
      }
    }
  ],
  "issue": {
    "commited": false,
    "id": 30451,
    "owner": "tsepez@chromium.org",
    "patchsets": [
      {
        "id": 1,
        "tryjobs": null
      },
      {
        "id": 2,
        "tryjobs": [
          {
            "buildBucketID": 8949431058799977968,
            "builder": "win_xfa_jumbo",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 2,
            "status": "failed"
          },
          {
            "buildBucketID": 8949431058799977984,
            "builder": "linux_xfa_jumbo",
            "issueID": 30451,
            "masterCommit": "822886b0c4478eb339fc5e2ec89f3fbdd78d57be",
            "patchsetID": 2,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949431058799978000,
            "builder": "linux",
            "issueID": 30451,
            "masterCommit": "822886b0c4478eb339fc5e2ec89f3fbdd78d57be",
            "patchsetID": 2,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949431058799978016,
            "builder": "linux_msan",
            "issueID": 30451,
            "masterCommit": "822886b0c4478eb339fc5e2ec89f3fbdd78d57be",
            "patchsetID": 2,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949431058799978032,
            "builder": "linux_xfa_asan_lsan",
            "issueID": 30451,
            "masterCommit": "822886b0c4478eb339fc5e2ec89f3fbdd78d57be",
            "patchsetID": 2,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949431058799978048,
            "builder": "win_xfa_rel",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 2,
            "status": "failed"
          },
          {
            "buildBucketID": 8949431058799978064,
            "builder": "mac_no_v8",
            "issueID": 30451,
            "masterCommit": "822886b0c4478eb339fc5e2ec89f3fbdd78d57be",
            "patchsetID": 2,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949431058799978080,
            "builder": "linux_skia",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 2,
            "status": "complete"
          },
          {
            "buildBucketID": 8949431058799978096,
            "builder": "mac_xfa_rel",
            "issueID": 30451,
            "masterCommit": "822886b0c4478eb339fc5e2ec89f3fbdd78d57be",
            "patchsetID": 2,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949431058799978112,
            "builder": "mac_xfa_jumbo",
            "issueID": 30451,
            "masterCommit": "822886b0c4478eb339fc5e2ec89f3fbdd78d57be",
            "patchsetID": 2,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949431058799978128,
            "builder": "win_xfa_msvc_32",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 2,
            "status": "failed"
          },
          {
            "buildBucketID": 8949431058799978144,
            "builder": "win_xfa_32",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 2,
            "status": "failed"
          },
          {
            "buildBucketID": 8949431058799978160,
            "builder": "win_asan",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 2,
            "status": "failed"
          },
          {
            "buildBucketID": 8949431058799978176,
            "builder": "mac",
            "issueID": 30451,
            "masterCommit": "822886b0c4478eb339fc5e2ec89f3fbdd78d57be",
            "patchsetID": 2,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949431058799978192,
            "builder": "android",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 2,
            "status": "complete"
          },
          {
            "buildBucketID": 8949431058799978208,
            "builder": "linux_no_v8",
            "issueID": 30451,
            "masterCommit": "822886b0c4478eb339fc5e2ec89f3fbdd78d57be",
            "patchsetID": 2,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949431058799978224,
            "builder": "win_no_v8",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 2,
            "status": "failed"
          },
          {
            "buildBucketID": 8949431058799978240,
            "builder": "linux_xfa",
            "issueID": 30451,
            "masterCommit": "822886b0c4478eb339fc5e2ec89f3fbdd78d57be",
            "patchsetID": 2,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949431058799978256,
            "builder": "mac_skia",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 2,
            "status": "complete"
          },
          {
            "buildBucketID": 8949431058799978272,
            "builder": "linux_asan_lsan",
            "issueID": 30451,
            "masterCommit": "822886b0c4478eb339fc5e2ec89f3fbdd78d57be",
            "patchsetID": 2,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949431058799978288,
            "builder": "mac_xfa",
            "issueID": 30451,
            "masterCommit": "822886b0c4478eb339fc5e2ec89f3fbdd78d57be",
            "patchsetID": 2,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949431058799978304,
            "builder": "win",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 2,
            "status": "failed"
          },
          {
            "buildBucketID": 8949431058799978320,
            "builder": "linux_xfa_rel",
            "issueID": 30451,
            "masterCommit": "822886b0c4478eb339fc5e2ec89f3fbdd78d57be",
            "patchsetID": 2,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949431058799978336,
            "builder": "win_xfa_msvc",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 2,
            "status": "failed"
          },
          {
            "buildBucketID": 8949431058799978352,
            "builder": "win_xfa",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 2,
            "status": "failed"
          },
          {
            "buildBucketID": 8949431058799978368,
            "builder": "linux_xfa_msan",
            "issueID": 30451,
            "masterCommit": "822886b0c4478eb339fc5e2ec89f3fbdd78d57be",
            "patchsetID": 2,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949431058799978384,
            "builder": "win_skia",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 2,
            "status": "failed"
          },
          {
            "buildBucketID": 8949431058799978400,
            "builder": "win_xfa_asan",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 2,
            "status": "failed"
          }
        ]
      },
      {
        "id": 3,
        "tryjobs": [
          {
            "buildBucketID": 8949430345803234176,
            "builder": "win_xfa_jumbo",
            "issueID": 30451,
            "masterCommit": "7f821c11081fe90346823333622253ec7949b583",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234192,
            "builder": "linux_xfa_jumbo",
            "issueID": 30451,
            "masterCommit": "d7f24d5182df335aab8042e1f71f6e402c427e4b",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234208,
            "builder": "linux",
            "issueID": 30451,
            "masterCommit": "d7f24d5182df335aab8042e1f71f6e402c427e4b",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234224,
            "builder": "linux_msan",
            "issueID": 30451,
            "masterCommit": "d7f24d5182df335aab8042e1f71f6e402c427e4b",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234240,
            "builder": "linux_xfa_asan_lsan",
            "issueID": 30451,
            "masterCommit": "d7f24d5182df335aab8042e1f71f6e402c427e4b",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234256,
            "builder": "win_xfa_rel",
            "issueID": 30451,
            "masterCommit": "7f821c11081fe90346823333622253ec7949b583",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234272,
            "builder": "mac_no_v8",
            "issueID": 30451,
            "masterCommit": "d7f24d5182df335aab8042e1f71f6e402c427e4b",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234288,
            "builder": "linux_skia",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 3,
            "status": "complete"
          },
          {
            "buildBucketID": 8949430345803234304,
            "builder": "mac_xfa_rel",
            "issueID": 30451,
            "masterCommit": "d7f24d5182df335aab8042e1f71f6e402c427e4b",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234320,
            "builder": "mac_xfa_jumbo",
            "issueID": 30451,
            "masterCommit": "d7f24d5182df335aab8042e1f71f6e402c427e4b",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234336,
            "builder": "win_xfa_msvc_32",
            "issueID": 30451,
            "masterCommit": "7f821c11081fe90346823333622253ec7949b583",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234352,
            "builder": "win_xfa_32",
            "issueID": 30451,
            "masterCommit": "7f821c11081fe90346823333622253ec7949b583",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234368,
            "builder": "win_asan",
            "issueID": 30451,
            "masterCommit": "7f821c11081fe90346823333622253ec7949b583",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234384,
            "builder": "mac",
            "issueID": 30451,
            "masterCommit": "d7f24d5182df335aab8042e1f71f6e402c427e4b",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234400,
            "builder": "android",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 3,
            "status": "complete"
          },
          {
            "buildBucketID": 8949430345803234416,
            "builder": "linux_no_v8",
            "issueID": 30451,
            "masterCommit": "d7f24d5182df335aab8042e1f71f6e402c427e4b",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234432,
            "builder": "win_no_v8",
            "issueID": 30451,
            "masterCommit": "7f821c11081fe90346823333622253ec7949b583",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234448,
            "builder": "linux_xfa",
            "issueID": 30451,
            "masterCommit": "d7f24d5182df335aab8042e1f71f6e402c427e4b",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234464,
            "builder": "mac_skia",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 3,
            "status": "complete"
          },
          {
            "buildBucketID": 8949430345803234480,
            "builder": "linux_asan_lsan",
            "issueID": 30451,
            "masterCommit": "d7f24d5182df335aab8042e1f71f6e402c427e4b",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234496,
            "builder": "mac_xfa",
            "issueID": 30451,
            "masterCommit": "d7f24d5182df335aab8042e1f71f6e402c427e4b",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234512,
            "builder": "win",
            "issueID": 30451,
            "masterCommit": "7f821c11081fe90346823333622253ec7949b583",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234528,
            "builder": "linux_xfa_rel",
            "issueID": 30451,
            "masterCommit": "6998bc502dd2798115024c48b95e6e9180b2b3ee",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234544,
            "builder": "win_xfa_msvc",
            "issueID": 30451,
            "masterCommit": "7f821c11081fe90346823333622253ec7949b583",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234560,
            "builder": "win_xfa",
            "issueID": 30451,
            "masterCommit": "7f821c11081fe90346823333622253ec7949b583",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234576,
            "builder": "linux_xfa_msan",
            "issueID": 30451,
            "masterCommit": "d7f24d5182df335aab8042e1f71f6e402c427e4b",
            "patchsetID": 3,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949430345803234592,
            "builder": "win_skia",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 3,
            "status": "complete"
          },
          {
            "buildBucketID": 8949430345803234608,
            "builder": "win_xfa_asan",
            "issueID": 30451,
            "masterCommit": "7f821c11081fe90346823333622253ec7949b583",
            "patchsetID": 3,
            "status": "ingested"
          }
        ]
      },
      {
        "id": 4,
        "tryjobs": null
      },
      {
        "id": 5,
        "tryjobs": [
          {
            "buildBucketID": 8949424578600524624,
            "builder": "win_xfa_jumbo",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600524640,
            "builder": "linux_xfa_jumbo",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600524656,
            "builder": "linux",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600524672,
            "builder": "linux_msan",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600524688,
            "builder": "linux_xfa_asan_lsan",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600524704,
            "builder": "win_xfa_rel",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600524720,
            "builder": "mac_no_v8",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600524736,
            "builder": "linux_skia",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600524752,
            "builder": "mac_xfa_rel",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600524768,
            "builder": "mac_xfa_jumbo",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600524784,
            "builder": "win_xfa_msvc_32",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600524800,
            "builder": "win_xfa_32",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600524816,
            "builder": "win_asan",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600524832,
            "builder": "mac",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600524848,
            "builder": "android",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "complete"
          },
          {
            "buildBucketID": 8949424578600524864,
            "builder": "linux_no_v8",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 5,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949424578600524880,
            "builder": "win_no_v8",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 5,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949424578600524896,
            "builder": "linux_xfa",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600524912,
            "builder": "mac_skia",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600524928,
            "builder": "linux_asan_lsan",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600524944,
            "builder": "mac_xfa",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600524960,
            "builder": "win",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600524976,
            "builder": "linux_xfa_rel",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600524992,
            "builder": "win_xfa_msvc",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600525008,
            "builder": "win_xfa",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600525024,
            "builder": "linux_xfa_msan",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600525040,
            "builder": "win_skia",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          },
          {
            "buildBucketID": 8949424578600525056,
            "builder": "win_xfa_asan",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 5,
            "status": "failed"
          }
        ]
      },
      {
        "id": 6,
        "tryjobs": [
          {
            "buildBucketID": 8949423540142356240,
            "builder": "win_xfa_jumbo",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949423540142356256,
            "builder": "linux_xfa_jumbo",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949423540142356272,
            "builder": "linux",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949423540142356288,
            "builder": "linux_msan",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949423540142356304,
            "builder": "linux_xfa_asan_lsan",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 6,
            "status": "running"
          },
          {
            "buildBucketID": 8949423540142356320,
            "builder": "win_xfa_rel",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949423540142356336,
            "builder": "mac_no_v8",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949423540142356352,
            "builder": "linux_skia",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 6,
            "status": "complete"
          },
          {
            "buildBucketID": 8949423540142356368,
            "builder": "mac_xfa_rel",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949423540142356384,
            "builder": "mac_xfa_jumbo",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949423540142356400,
            "builder": "win_xfa_msvc_32",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949423540142356416,
            "builder": "win_xfa_32",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949423540142356432,
            "builder": "win_asan",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949423540142356448,
            "builder": "mac",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949423540142356464,
            "builder": "android",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 6,
            "status": "complete"
          },
          {
            "buildBucketID": 8949423540142356480,
            "builder": "linux_no_v8",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949423540142356496,
            "builder": "win_no_v8",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949423540142356512,
            "builder": "linux_xfa",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949423540142356528,
            "builder": "mac_skia",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 6,
            "status": "complete"
          },
          {
            "buildBucketID": 8949423540142356544,
            "builder": "linux_asan_lsan",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949423540142356560,
            "builder": "mac_xfa",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949423540142356576,
            "builder": "win",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949423540142356592,
            "builder": "linux_xfa_rel",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949423540142356608,
            "builder": "win_xfa_msvc",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949423540142356624,
            "builder": "win_xfa",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949423540142356640,
            "builder": "linux_xfa_msan",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          },
          {
            "buildBucketID": 8949423540142356656,
            "builder": "win_skia",
            "issueID": 30451,
            "masterCommit": "",
            "patchsetID": 6,
            "status": "complete"
          },
          {
            "buildBucketID": 8949423540142356672,
            "builder": "win_xfa_asan",
            "issueID": 30451,
            "masterCommit": "154e18f9a862975abecebe77b8f5fb418418d14c",
            "patchsetID": 6,
            "status": "ingested"
          }
        ]
      }
    ],
    "queryPatchsets": [
      2
    ],
    "status": "NEW",
    "subject": "Return pdfium::span<char> from ByteString::GetBuffer().",
    "updated": 1523561692000,
    "url": "https://pdfium-review.googlesource.com/c/30451"
  },
  "offset": 0,
  "size": 1
};


// "issue": {
//   "id": "1953533002",
//   "owner": "mtklein_C",
//   "patchsets": [
//     {
//       "digests": 0,
//       "id": 1,
//       "inMaster": 0,
//       "jobDone": 13,
//       "jobTotal": 13,
//       "tryjobs": [
//         {
//           "builder": "Test-Ubuntu-GCC-GCE-CPU-AVX2-x86_64-Release-Shared-Trybot",
//           "buildnumber": "",
//           "status": "ingested"
//         },
//         {
//           "builder": "Test-Ubuntu-GCC-ShuttleA-GPU-GTX660-x86_64-Release-Trybot",
//           "buildnumber": "",
//           "status": "ingested"
//         },
//         {
//           "builder": "Test-Ubuntu-GCC-GCE-CPU-AVX2-x86_64-Debug-Trybot",
//           "buildnumber": "",
//           "status": "failed"
//         }
//       ],
//       "url": ""
//     },
//     {
//       "digests": 0,
//       "id": 20001,
//       "inMaster": 0,
//       "jobDone": 14,
//       "jobTotal": 14,
//       "tryjobs": [
//         {
//           "builder": "Test-Ubuntu-GCC-GCE-CPU-AVX2-x86_64-Release-SKNX_NO_SIMD-Trybot",
//           "buildnumber": "",
//           "status": "ingested"
//         },
//         {
//           "builder": "Test-Ubuntu-GCC-GCE-CPU-AVX2-x86_64-Release-Shared-Trybot",
//           "buildnumber": "",
//           "status": "failed"
//         },
//         {
//           "builder": "Test-Ubuntu-GCC-ShuttleA-GPU-GTX660-x86_64-Release-Trybot",
//           "buildnumber": "",
//           "status": "ingested"
//         },
//         {
//           "builder": "Test-Ubuntu-GCC-GCE-CPU-AVX2-x86_64-Debug-Trybot",
//           "buildnumber": "",
//           "status": "ingested"
//         }
//       ],
//       "url": ""
//     },
//     {
//       "digests": 0,
//       "id": 40001,
//       "inMaster": 0,
//       "jobDone": 14,
//       "jobTotal": 14,
//       "tryjobs": [
//         {
//           "builder": "Test-Ubuntu-GCC-GCE-CPU-AVX2-x86_64-Release-SKNX_NO_SIMD-Trybot",
//           "buildnumber": "",
//           "status": "ingested"
//         },
//         {
//           "builder": "Test-Ubuntu-GCC-GCE-CPU-AVX2-x86_64-Release-Shared-Trybot",
//           "buildnumber": "",
//           "status": "ingested"
//         },
//         {
//           "builder": "Test-Ubuntu-GCC-ShuttleA-GPU-GTX660-x86_64-Release-Trybot",
//           "buildnumber": "",
//           "status": "ingested"
//         },
//         {
//           "builder": "Test-Ubuntu-GCC-GCE-CPU-AVX2-x86_64-Debug-Trybot",
//           "buildnumber": "",
//           "status": "failed"
//         }
//       ],
//       "url": ""
//     },
//     {
//       "digests": 0,
//       "id": 60001,
//       "inMaster": 0,
//       "jobDone": 15,
//       "jobTotal": 15,
//       "tryjobs": [
//         {
//           "builder": "Test-Ubuntu-GCC-GCE-CPU-AVX2-x86_64-Release-SKNX_NO_SIMD-Trybot",
//           "buildnumber": "",
//           "status": "running"
//         },
//         {
//           "builder": "Test-Ubuntu-GCC-GCE-CPU-AVX2-x86_64-Release-Shared-Trybot",
//           "buildnumber": "",
//           "status": "complete"
//         },
//         {
//           "builder": "Test-Ubuntu-GCC-ShuttleA-GPU-GTX660-x86_64-Release-Trybot",
//           "buildnumber": "",
//           "status": "ingested"
//         },
//         {
//           "builder": "Test-Ubuntu-GCC-GCE-CPU-AVX2-x86_64-Debug-Trybot",
//           "buildnumber": "",
//           "status": "failed"
//         }
//       ],
//       "url": ""
//     }
//   ],
//   "queryPatchsets": [
//     "60001"
//   ],
//   "subject": "SkOncePtr -> SkOnce",
//   "updated": 1462455882,
//   "url": "https://codereview.chromium.org/1953533002"
// }
