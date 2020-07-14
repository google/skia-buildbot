import { Status } from './arb-status-sk';

export const fakeStatus: Status = JSON.parse(String.raw`{
    "childHead": "",
    "childName": "Skia",
    "config": {
      "childDisplayName": "Skia",
      "commitMsg": {
        "bugProject": "chromium",
        "childLogURLTmpl": "https://skia.googlesource.com/skia.git/+log/{{.RollingFrom}}..{{.RollingTo}}",
        "cqExtraTrybots": [
          "luci.chromium.try:android_optional_gpu_tests_rel",
          "luci.chromium.try:linux-blink-rel",
          "luci.chromium.try:linux-chromeos-compile-dbg",
          "luci.chromium.try:linux_optional_gpu_tests_rel",
          "luci.chromium.try:mac_optional_gpu_tests_rel",
          "luci.chromium.try:win_optional_gpu_tests_rel"
        ],
        "includeLog": true,
        "includeRevisionCount": true,
        "includeTbrLine": true,
        "includeTests": true
      },
      "contacts": [
        "borenet@google.com"
      ],
      "gerrit": {
        "config": "chromium",
        "project": "chromium/src",
        "url": "https://chromium-review.googlesource.com"
      },
      "isInternal": false,
      "kubernetes": {
        "cpu": "1",
        "memory": "2Gi",
        "readinessFailureThreshold": "10",
        "readinessInitialDelaySeconds": "30",
        "readinessPeriodSeconds": "30"
      },
      "maxRollFrequency": "0m",
      "noCheckoutDEPSRepoManager": {
        "childBranch": "master",
        "childPath": "src/third_party/skia",
        "childRepo": "https://skia.googlesource.com/skia.git",
        "childRevLinkTmpl": "https://skia.googlesource.com/skia.git/+show/%s",
        "gerrit": {
          "config": "chromium",
          "project": "chromium/src",
          "url": "https://chromium-review.googlesource.com"
        },
        "parentBranch": "master",
        "parentRepo": "https://chromium.googlesource.com/chromium/src.git"
      },
      "notifiers": [
        {
          "email": {
            "emails": [
              "borenet@google.com"
            ]
          },
          "filter": "warning"
        }
      ],
      "ownerPrimary": "borenet",
      "ownerSecondary": "rmistry",
      "parentDisplayName": "Chromium",
      "parentWaterfall": "https://build.chromium.org",
      "rollerName": "skia-autoroll",
      "serviceAccount": "chromium-autoroll@skia-public.iam.gserviceaccount.com",
      "sheriff": [
        "https://tree-status.skia.org/current-sheriff"
      ],
      "supportsManualRolls": true
    },
    "currentRoll": {
      "closed": false,
      "comments": null,
      "committed": false,
      "cqFinished": false,
      "cqSuccess": false,
      "created": "2020-07-10T14:55:00Z",
      "dryRunFinished": false,
      "dryRunSuccess": false,
      "isDryRun": false,
      "issue": 2292362,
      "modified": "2020-07-10T14:55:25Z",
      "patchSets": [
        1,
        2
      ],
      "result": "in progress",
      "rollingFrom": "f8a6b5b4b0d02895f70af4158c47c3069488a64a",
      "rollingTo": "082323b57da7c15e9d43ea3b68d2e6907e9b8139",
      "subject": "Roll Skia from f8a6b5b4b0d0 to 082323b57da7 (8 revisions)",
      "tryResults": [
        {
          "builder": "android-binary-size",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175571136"
        },
        {
          "builder": "android-lollipop-arm-rel",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175571120"
        },
        {
          "builder": "android-marshmallow-arm64-rel",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175571104"
        },
        {
          "builder": "android-pie-arm64-rel",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175571088"
        },
        {
          "builder": "android_compile_dbg",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175571072"
        },
        {
          "builder": "android_cronet",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175571056"
        },
        {
          "builder": "android_optional_gpu_tests_rel",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175571040"
        },
        {
          "builder": "cast_shell_android",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175571024"
        },
        {
          "builder": "cast_shell_linux",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175571008"
        },
        {
          "builder": "chromeos-amd64-generic-rel",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570992"
        },
        {
          "builder": "chromeos-arm-generic-rel",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570976"
        },
        {
          "builder": "chromium_presubmit",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570960"
        },
        {
          "builder": "fuchsia-compile-x64-dbg",
          "category": "cq_experimental",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570944"
        },
        {
          "builder": "fuchsia-x64-cast",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570928"
        },
        {
          "builder": "fuchsia_arm64",
          "category": "cq_experimental",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570912"
        },
        {
          "builder": "fuchsia_x64",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570896"
        },
        {
          "builder": "ios-simulator",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570880"
        },
        {
          "builder": "linux-blink-rel",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570864"
        },
        {
          "builder": "linux-chromeos-compile-dbg",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570848"
        },
        {
          "builder": "linux-chromeos-rel",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570832"
        },
        {
          "builder": "linux-libfuzzer-asan-rel",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570816"
        },
        {
          "builder": "linux-ozone-rel",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570800"
        },
        {
          "builder": "linux-rel",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570784"
        },
        {
          "builder": "linux_chromium_asan_rel_ng",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570768"
        },
        {
          "builder": "linux_chromium_compile_dbg_ng",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570752"
        },
        {
          "builder": "linux_chromium_tsan_rel_ng",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570736"
        },
        {
          "builder": "linux_optional_gpu_tests_rel",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570720"
        },
        {
          "builder": "mac-rel",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "",
          "status": "STARTED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570704"
        },
        {
          "builder": "mac_chromium_compile_dbg_ng",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570688"
        },
        {
          "builder": "mac_optional_gpu_tests_rel",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570672"
        },
        {
          "builder": "win-libfuzzer-asan-rel",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570656"
        },
        {
          "builder": "win10_chromium_x64_rel_ng",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "",
          "status": "STARTED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570640"
        },
        {
          "builder": "win_chromium_compile_dbg_ng",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570624"
        },
        {
          "builder": "win_optional_gpu_tests_rel",
          "category": "cq",
          "created_ts": "2020-07-10T14:55:35.546417Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875151549175570608"
        }
      ]
    },
    "currentRollRev": "082323b57da7c15e9d43ea3b68d2e6907e9b8139",
    "error": "",
    "fullHistoryUrl": "https://chromium-review.googlesource.com/q/owner:chromium-autoroll@skia-public.iam.gserviceaccount.com",
    "issueUrlBase": "https://chromium-review.googlesource.com/c/",
    "lastRoll": {
      "closed": true,
      "comments": null,
      "committed": true,
      "cqFinished": true,
      "cqSuccess": true,
      "created": "2020-07-10T13:10:00Z",
      "dryRunFinished": false,
      "dryRunSuccess": false,
      "isDryRun": false,
      "issue": 2292355,
      "modified": "2020-07-10T14:52:11Z",
      "patchSets": [
        1,
        2,
        3
      ],
      "result": "succeeded",
      "rollingFrom": "9f821489c9f39c53bba496217cd7c1cf2ae9742b",
      "rollingTo": "f8a6b5b4b0d02895f70af4158c47c3069488a64a",
      "subject": "Roll Skia from 9f821489c9f3 to f8a6b5b4b0d0 (1 revision)",
      "tryResults": [
        {
          "builder": "android-binary-size",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557026080"
        },
        {
          "builder": "android-lollipop-arm-rel",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557026064"
        },
        {
          "builder": "android-marshmallow-arm64-rel",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557026048"
        },
        {
          "builder": "android-pie-arm64-rel",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557026032"
        },
        {
          "builder": "android_compile_dbg",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557026016"
        },
        {
          "builder": "android_cronet",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557026000"
        },
        {
          "builder": "android_optional_gpu_tests_rel",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025984"
        },
        {
          "builder": "cast_shell_android",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025968"
        },
        {
          "builder": "cast_shell_linux",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025952"
        },
        {
          "builder": "chromeos-amd64-generic-rel",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025936"
        },
        {
          "builder": "chromeos-arm-generic-rel",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025920"
        },
        {
          "builder": "chromium_presubmit",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025904"
        },
        {
          "builder": "fuchsia-compile-x64-dbg",
          "category": "cq_experimental",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025888"
        },
        {
          "builder": "fuchsia-x64-cast",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025872"
        },
        {
          "builder": "fuchsia_arm64",
          "category": "cq_experimental",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025856"
        },
        {
          "builder": "fuchsia_x64",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025840"
        },
        {
          "builder": "ios-simulator",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025824"
        },
        {
          "builder": "linux-blink-rel",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025808"
        },
        {
          "builder": "linux-chromeos-compile-dbg",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025792"
        },
        {
          "builder": "linux-chromeos-rel",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025776"
        },
        {
          "builder": "linux-libfuzzer-asan-rel",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025760"
        },
        {
          "builder": "linux-ozone-rel",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025744"
        },
        {
          "builder": "linux-rel",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025728"
        },
        {
          "builder": "linux_chromium_asan_rel_ng",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025712"
        },
        {
          "builder": "linux_chromium_compile_dbg_ng",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025696"
        },
        {
          "builder": "linux_chromium_tsan_rel_ng",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025680"
        },
        {
          "builder": "linux_optional_gpu_tests_rel",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025664"
        },
        {
          "builder": "mac-rel",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025648"
        },
        {
          "builder": "mac_chromium_compile_dbg_ng",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025632"
        },
        {
          "builder": "mac_optional_gpu_tests_rel",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025616"
        },
        {
          "builder": "win-libfuzzer-asan-rel",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025600"
        },
        {
          "builder": "win10_chromium_x64_rel_ng",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025584"
        },
        {
          "builder": "win_chromium_compile_dbg_ng",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025568"
        },
        {
          "builder": "win_optional_gpu_tests_rel",
          "category": "cq",
          "created_ts": "2020-07-10T13:10:28.534039Z",
          "result": "SUCCESS",
          "status": "COMPLETED",
          "url": "https://cr-buildbucket.appspot.com/build/8875158162557025552"
        }
      ]
    },
    "lastRollRev": "f8a6b5b4b0d02895f70af4158c47c3069488a64a",
    "manualRequests": [
      {
        "id": "XYr3VprlfCVUu376WhDI",
        "requester": "skia-autoroll",
        "result": "SUCCESS",
        "revision": "refs/changes/80/300780/44",
        "rollerName": "skia-autoroll",
        "status": "COMPLETED",
        "timestamp": "2020-07-10T10:49:37.862775Z",
        "url": "https://chromium-review.googlesource.com/c/2292135"
      },
      {
        "id": "fELJSbxj3bK4o8e4DEhQ",
        "requester": "skia-autoroll",
        "result": "SUCCESS",
        "revision": "refs/changes/80/300780/42",
        "rollerName": "skia-autoroll",
        "status": "COMPLETED",
        "timestamp": "2020-07-09T20:43:58.506073Z",
        "url": "https://chromium-review.googlesource.com/c/2290491"
      },
      {
        "id": "FO6rPMV6OWooVWZQifOT",
        "requester": "skia-autoroll",
        "result": "FAILURE",
        "revision": "refs/changes/76/264776/6",
        "rollerName": "skia-autoroll",
        "status": "COMPLETED",
        "timestamp": "2020-07-09T20:35:28.125245Z",
        "url": "https://chromium-review.googlesource.com/c/2290420"
      },
      {
        "id": "w9AOr2TlSZwYoFZicV7h",
        "requester": "skia-autoroll",
        "result": "FAILURE",
        "revision": "refs/changes/76/264776/6",
        "rollerName": "skia-autoroll",
        "status": "COMPLETED",
        "timestamp": "2020-07-09T20:31:20.406932Z",
        "url": "https://chromium-review.googlesource.com/c/2290419"
      },
      {
        "id": "Eln2naPJvmC01AErSibq",
        "requester": "skia-autoroll",
        "result": "SUCCESS",
        "revision": "refs/changes/80/300780/41",
        "rollerName": "skia-autoroll",
        "status": "COMPLETED",
        "timestamp": "2020-07-09T20:12:46.142808Z",
        "url": "https://chromium-review.googlesource.com/c/2290276"
      },
      {
        "id": "rkpfHcYSLNpFGGDg5PCJ",
        "requester": "rmistry@google.com",
        "result": "FAILURE",
        "revision": "refs/changes/76/264776/5",
        "rollerName": "skia-autoroll",
        "status": "COMPLETED",
        "timestamp": "2020-07-05T15:29:35.702292Z",
        "url": "https://chromium-review.googlesource.com/c/2281901"
      },
      {
        "id": "GiGkeot4GZuAvC2yXuJm",
        "requester": "rmistry@google.com",
        "result": "FAILURE",
        "revision": "refs/changes/76/264776/5",
        "rollerName": "skia-autoroll",
        "status": "COMPLETED",
        "timestamp": "2020-07-05T15:28:53.753463Z",
        "url": "https://chromium-review.googlesource.com/c/2281902"
      },
      {
        "id": "FqhQJmnVNupDaAb38Urb",
        "requester": "egdaniel@google.com",
        "result": "FAILURE",
        "revision": "aed25a93a42406aef01c80351d14834781b738e8",
        "rollerName": "skia-autoroll",
        "status": "COMPLETED",
        "timestamp": "2020-06-24T18:51:17.425615Z",
        "url": "https://chromium-review.googlesource.com/c/2265152"
      },
      {
        "id": "ruTSTPc24HsQ1IrrGmPT",
        "requester": "egdaniel@google.com",
        "result": "FAILURE",
        "revision": "c5f25bc215e4a23fda887c81d177303f26f2118e",
        "rollerName": "skia-autoroll",
        "status": "COMPLETED",
        "timestamp": "2020-06-24T18:51:15.457022Z",
        "url": "https://chromium-review.googlesource.com/c/2264917"
      },
      {
        "id": "MFxX3WtKVGKdZVVnlQh6",
        "requester": "egdaniel@google.com",
        "result": "FAILURE",
        "revision": "7d7cd2b17877c0bf39c9d15d384c781e9d92e719",
        "rollerName": "skia-autoroll",
        "status": "COMPLETED",
        "timestamp": "2020-06-24T18:51:14.270958Z",
        "url": "https://chromium-review.googlesource.com/c/2265153"
      },
      {
        "id": "wue5CIhcgMcwagZCgTek",
        "requester": "egdaniel@google.com",
        "result": "FAILURE",
        "revision": "3b6b7478421b4819e0f5ea08c44d90818b2cd739",
        "rollerName": "skia-autoroll",
        "status": "COMPLETED",
        "timestamp": "2020-06-24T18:51:12.954572Z",
        "url": "https://chromium-review.googlesource.com/c/2265154"
      },
      {
        "id": "Pu3yzs1sFPHcNCMSTctU",
        "requester": "egdaniel@google.com",
        "result": "FAILURE",
        "revision": "d34528c3576bccef9681e4fe947c825355782a1a",
        "rollerName": "skia-autoroll",
        "status": "COMPLETED",
        "timestamp": "2020-06-24T18:51:11.243028Z",
        "url": "https://chromium-review.googlesource.com/c/2265155"
      }
    ],
    "mode": {
      "message": "",
      "mode": "running",
      "time": "2020-06-26T14:41:00.070874Z",
      "user": "mtklein@google.com"
    },
    "notRolledRevs": [
      {
        "author": "robertphillips@google.com",
        "bugs": {},
        "dependencies": null,
        "description": "Switch a few GMs over to GrDirectContext",
        "details": "Change-Id: I96684e0c3a36e194c0ce68b32f09aab2b6e5b625\nReviewed-on: https://skia-review.googlesource.com/c/skia/+/301860\nReviewed-by: Adlai Holler <adlai@google.com>\nCommit-Queue: Robert Phillips <robertphillips@google.com>\n",
        "display": "d436b78ad4be",
        "id": "d436b78ad4be6db6675c7244b9bf3121028e6dc1",
        "invalidReason": "",
        "tests": null,
        "time": "2020-07-10T16:23:03Z",
        "url": "https://skia.googlesource.com/skia.git/+show/d436b78ad4be6db6675c7244b9bf3121028e6dc1"
      },
      {
        "author": "nigeltao@google.com",
        "bugs": {},
        "dependencies": null,
        "description": "Use Wuffs v0.3 by default, not v0.2",
        "details": "This is roll-forward of\nhttps://skia-review.googlesource.com/c/skia/+/298616/\n\nChange-Id: I44d65dcb67555fd76075e2d415d96db82c376cae\nReviewed-on: https://skia-review.googlesource.com/c/skia/+/301756\nCommit-Queue: Leon Scroggins <scroggo@google.com>\nReviewed-by: Leon Scroggins <scroggo@google.com>\n",
        "display": "c91db040ad18",
        "id": "c91db040ad18b9cc3236e342e9acca020eaafd10",
        "invalidReason": "",
        "tests": null,
        "time": "2020-07-10T16:10:43Z",
        "url": "https://skia.googlesource.com/skia.git/+show/c91db040ad18b9cc3236e342e9acca020eaafd10"
      },
      {
        "author": "zepenghu@google.com",
        "bugs": {},
        "dependencies": null,
        "description": "Add SkRuntimeEffect Fuzzer",
        "details": "The major improvement is that now the fuzzer is able to execute\nthe sksl code (before it just compiled it). The fuzzer will\nreserve 256 bytes for providing uniforms to the shader;\nmeanwhile, the fuzzer will read the remaining bytes as sksl code\nto create SkRuntimeEffect. It then creates a shader and executes\nit by painting the shader on a canvas.\n\nThe code was tested locally with afl-fuzz, and the execution \nspeed was around 700/sec.\n\nAn alternative implementation would have been using Fuzz.h to\nread bytes; I decided to go with sk_sp<SkData> since it has a\ncomparable format to other binary fuzzer and meets all the\nfunctionality in this fuzzer.\n\nFor future changes, there are 2 important improvements to the\nimplementation:\n\n1) Current shader does not have children shaders; thus,\nmakeShader() will fail if the SkSL ever tries to use an 'in shader'.\n\nAs pointed out in patchset 11, after creating the runtime effect,\neffect->children().count() will tell you how many children it's\nexpecting (how many 'in shader' variables were declared). When you\ncall makeShader(), the second and third arguments are a\n(C-style) array of shader pointers, and\na count (which must match children().count()).\n\nSome helpful examples can be SkRTShader::CreateProc in\nSkRuntimeEffect.cpp, make_fuzz_shader in FuzzCanvas.cpp.\n\n2)\n\nIn this fuzzer, after creating the paint from a shader, the paint\ncan be drawn on either GPU canvas or CPU, so a possible way is to\nuse SkSurface::MakeRenderTarget to create GPU canvas and use a byte\nto determine which canvas it will be drawn on.\n\nChange-Id: Ib0385edd0f5ec2f23744aa517135a6955c53ba38\nReviewed-on: https://skia-review.googlesource.com/c/skia/+/300618\nCommit-Queue: Zepeng Hu <zepenghu@google.com>\nReviewed-by: Brian Osman <brianosman@google.com>\nReviewed-by: Kevin Lubick <kjlubick@google.com>\n",
        "display": "a5783f3858ba",
        "id": "a5783f3858ba528544d2a47b34ca726e0e805369",
        "invalidReason": "",
        "tests": null,
        "time": "2020-07-10T15:54:23Z",
        "url": "https://skia.googlesource.com/skia.git/+show/a5783f3858ba528544d2a47b34ca726e0e805369"
      },
      {
        "author": "mtklein@google.com",
        "bugs": {},
        "dependencies": null,
        "description": "Revert \"Exclude gl files for Fuchsia platform.\"",
        "details": "This reverts commit c1f3f8cda4c9cb35ea01385f8a5d842bb176682d.\n\nReason for revert:  duplicate symbols on other platforms\n\nld.lld: error: duplicate symbol: GrGLCreateNativeInterface()\n>>> defined at GrGLMakeNativeInterface_none.cpp:12 (third_party/skia/HEAD/src/gpu/gl/GrGLMakeNativeInterface_none.cpp:12)\n>>>            blaze-out/arm64-v8a-fastbuild/bin/third_party/skia/HEAD/_objs/skia/GrGLMakeNativeInterface_none.pic.o:(GrGLCreateNativeInterface())\n>>> defined at GrGLMakeNativeInterface_egl.cpp:137 (third_party/skia/HEAD/src/gpu/gl/android/../egl/GrGLMakeNativeInterface_egl.cpp:137)\n>>>            blaze-out/arm64-v8a-fastbuild/bin/third_party/skia/HEAD/_objs/skia/GrGLMakeNativeInterface_android.pic.o:(.text._Z25GrGLCreateNativeInterfacev+0x0)\n\nI even ran the G3 trybot and forgot to see if it was green...\n\nOriginal change's description:\n> Exclude gl files for Fuchsia platform.\n> \n> Fuchsia platform supports only vulkan. So exclude all gl files and\n> add vulkan files for building Skia on Fuchsia.\n> \n> Change-Id: I2593a14926747b1154a1134bfdd43772627110a4\n> Reviewed-on: https://skia-review.googlesource.com/c/skia/+/301739\n> Reviewed-by: Mike Klein <mtklein@google.com>\n> Reviewed-by: Brian Salomon <bsalomon@google.com>\n> Commit-Queue: Mike Klein <mtklein@google.com>\n\nTBR=mtklein@google.com,bsalomon@google.com,guruji@google.com\n\nChange-Id: If2623c600c5b3fc43bf896b4da02dc7cf61d8a27\nNo-Presubmit: true\nNo-Tree-Checks: true\nNo-Try: true\nReviewed-on: https://skia-review.googlesource.com/c/skia/+/301896\nReviewed-by: Mike Klein <mtklein@google.com>\nCommit-Queue: Mike Klein <mtklein@google.com>\n",
        "display": "5160e8caa226",
        "id": "5160e8caa226213c77b2a5f98908aa4eeee75ef5",
        "invalidReason": "",
        "tests": null,
        "time": "2020-07-10T15:07:35Z",
        "url": "https://skia.googlesource.com/skia.git/+show/5160e8caa226213c77b2a5f98908aa4eeee75ef5"
      },
      {
        "author": "fmalita@chromium.org",
        "bugs": {},
        "dependencies": null,
        "description": "[skottie] Fill-over-stroke support for text",
        "details": "AE allows selecting the paint order when both fill & stroke are present.\n\nThe CL also fixes some text stroke issues: stroke width not parsed\ncorrectly and not actually used on the paint.\n\nChange-Id: Iec27bb65d09f689365e43b801d3844106780572b\nReviewed-on: https://skia-review.googlesource.com/c/skia/+/301857\nReviewed-by: Ben Wagner <bungeman@google.com>\nCommit-Queue: Florin Malita <fmalita@google.com>\nCommit-Queue: Florin Malita <fmalita@chromium.org>\n",
        "display": "082323b57da7",
        "id": "082323b57da7c15e9d43ea3b68d2e6907e9b8139",
        "invalidReason": "",
        "tests": null,
        "time": "2020-07-10T14:44:23Z",
        "url": "https://skia.googlesource.com/skia.git/+show/082323b57da7c15e9d43ea3b68d2e6907e9b8139"
      },
      {
        "author": "guruji@google.com",
        "bugs": {},
        "dependencies": null,
        "description": "Exclude gl files for Fuchsia platform.",
        "details": "Fuchsia platform supports only vulkan. So exclude all gl files and\nadd vulkan files for building Skia on Fuchsia.\n\nChange-Id: I2593a14926747b1154a1134bfdd43772627110a4\nReviewed-on: https://skia-review.googlesource.com/c/skia/+/301739\nReviewed-by: Mike Klein <mtklein@google.com>\nReviewed-by: Brian Salomon <bsalomon@google.com>\nCommit-Queue: Mike Klein <mtklein@google.com>\n",
        "display": "c1f3f8cda4c9",
        "id": "c1f3f8cda4c9cb35ea01385f8a5d842bb176682d",
        "invalidReason": "",
        "tests": null,
        "time": "2020-07-10T14:09:58Z",
        "url": "https://skia.googlesource.com/skia.git/+show/c1f3f8cda4c9cb35ea01385f8a5d842bb176682d"
      },
      {
        "author": "egdaniel@google.com",
        "bugs": {
          "chromium": [
            "1099255"
          ]
        },
        "dependencies": null,
        "description": "Add internal calls for updateCompressedBackendTexture.",
        "details": "This splits the creation and the updating of backend compressed textures\nbelow the public API level. In a follow on change I will add the public\napi call to updateCompressedBackendTexture.\n\nBug: chromium:1099255\nChange-Id: Ie410cfb42046d0e0c8f4fe60055be5782f811d47\nReviewed-on: https://skia-review.googlesource.com/c/skia/+/301577\nCommit-Queue: Greg Daniel <egdaniel@google.com>\nReviewed-by: Robert Phillips <robertphillips@google.com>\n",
        "display": "aaf738cf27fb",
        "id": "aaf738cf27fb77fac0c9222547348b50a42a15c9",
        "invalidReason": "",
        "tests": null,
        "time": "2020-07-10T14:08:53Z",
        "url": "https://skia.googlesource.com/skia.git/+show/aaf738cf27fb77fac0c9222547348b50a42a15c9"
      },
      {
        "author": "bungeman@google.com",
        "bugs": {},
        "dependencies": null,
        "description": "Fix Op tests when default typeface is empty.",
        "details": "While testing on Linux with a configuration like\n\nskia_enable_fontmgr_custom_directory=false\nskia_enable_fontmgr_custom_embedded=false\nskia_enable_fontmgr_custom_empty=true\nskia_enable_gpu=true\nskia_use_fontconfig=false\nskia_use_freetype=true\nskia_use_system_freetype2=false\n\nthe default typeface will be an empty typeface with no glyphs. This of\ncourse leads to many test failures, which is fine.\n\nHowever, this also leads to crashes when testing GPU Ops since the Op\nfactories may return nullptr to indicate no-op but the callers of those\nfactories currently do not expect nullptr or handle it as a no-op.\n\nChange the callers of Op factories to treat nullptr as no-op.\n\nChange-Id: I9eb1dfca4a8a9066a9cfb4c902d1f52d07763667\nReviewed-on: https://skia-review.googlesource.com/c/skia/+/301586\nReviewed-by: Herb Derby <herb@google.com>\nCommit-Queue: Ben Wagner <bungeman@google.com>\n",
        "display": "525e87682ceb",
        "id": "525e87682ceb0f009f34e7f81945bd3ba859687b",
        "invalidReason": "",
        "tests": null,
        "time": "2020-07-10T14:04:53Z",
        "url": "https://skia.googlesource.com/skia.git/+show/525e87682ceb0f009f34e7f81945bd3ba859687b"
      },
      {
        "author": "bsalomon@google.com",
        "bugs": {},
        "dependencies": null,
        "description": "Revert \"Revert \"Put top level FPs into their own functions\"\"",
        "details": "Now that we have inlining this should be ok.\n\nThis reverts commit 24dcd207ea7f9c5f03121663963c75b74ec759cb.\n\nChange-Id: I2aed6de6d962595cb0f3305bc26f340c99e0d1d2\nReviewed-on: https://skia-review.googlesource.com/c/skia/+/301552\nReviewed-by: Michael Ludwig <michaelludwig@google.com>\nCommit-Queue: Brian Salomon <bsalomon@google.com>\n",
        "display": "7a96c2a6bb37",
        "id": "7a96c2a6bb374b36ddf633b529c303e731e7467b",
        "invalidReason": "",
        "tests": null,
        "time": "2020-07-10T13:59:53Z",
        "url": "https://skia.googlesource.com/skia.git/+show/7a96c2a6bb374b36ddf633b529c303e731e7467b"
      },
      {
        "author": "michaelludwig@google.com",
        "bugs": {
          "chromium": [
            "1102578"
          ]
        },
        "dependencies": null,
        "description": "Apply paint color to alpha-only textures in drawEdgeAAImageSet",
        "details": "This makes batch texture ops consistent with singleton texture ops, and\nimages drawn through a texture producer.\n\nBug: chromium:1102578\nChange-Id: I490b20940ef6f1899396b786369271ce7130e8a9\nReviewed-on: https://skia-review.googlesource.com/c/skia/+/301540\nCommit-Queue: Michael Ludwig <michaelludwig@google.com>\nReviewed-by: Greg Daniel <egdaniel@google.com>\n",
        "display": "1c66ad940e5c",
        "id": "1c66ad940e5cf2a9edfe85b3cda0b112848e26d1",
        "invalidReason": "",
        "tests": null,
        "time": "2020-07-10T13:29:33Z",
        "url": "https://skia.googlesource.com/skia.git/+show/1c66ad940e5cf2a9edfe85b3cda0b112848e26d1"
      },
      {
        "author": "brianosman@google.com",
        "bugs": {},
        "dependencies": null,
        "description": "Add SkCodecImageGenerator::getScaledDimensions",
        "details": "Like SkCodec::getScaledDimensions, but accounts for orientation\n\nChange-Id: I53ba682d5b60e46053cf3cc50b7e6430929cfcef\nReviewed-on: https://skia-review.googlesource.com/c/skia/+/301581\nReviewed-by: Leon Scroggins <scroggo@google.com>\nCommit-Queue: Brian Osman <brianosman@google.com>\n",
        "display": "d9eb219e809a",
        "id": "d9eb219e809ac2e282f36c1e99a52cc1ae7522a8",
        "invalidReason": "",
        "tests": null,
        "time": "2020-07-10T13:16:23Z",
        "url": "https://skia.googlesource.com/skia.git/+show/d9eb219e809ac2e282f36c1e99a52cc1ae7522a8"
      },
      {
        "author": "mtklein@google.com",
        "bugs": {},
        "dependencies": null,
        "description": "pack windows ABI stack tightly",
        "details": "Reserve tight space for saving callee-saved registers,\nand skip any stack adjustment if we don't need it (again).\n\nChange-Id: Ieec82c2de8cf23db3ae4057435d4bdb4dd78c791\nReviewed-on: https://skia-review.googlesource.com/c/skia/+/301659\nAuto-Submit: Mike Klein <mtklein@google.com>\nReviewed-by: Herb Derby <herb@google.com>\nCommit-Queue: Mike Klein <mtklein@google.com>\n",
        "display": "377923e22955",
        "id": "377923e2295527b73d52bd9cabd05fdfac9849fb",
        "invalidReason": "",
        "tests": null,
        "time": "2020-07-10T13:10:13Z",
        "url": "https://skia.googlesource.com/skia.git/+show/377923e2295527b73d52bd9cabd05fdfac9849fb"
      }
    ],
    "numBehind": 12,
    "numFailed": 0,
    "parentName": "",
    "recent": [
      {
        "closed": false,
        "comments": null,
        "committed": false,
        "cqFinished": false,
        "cqSuccess": false,
        "created": "2020-07-10T14:55:00Z",
        "dryRunFinished": false,
        "dryRunSuccess": false,
        "isDryRun": false,
        "issue": 2292362,
        "modified": "2020-07-10T14:55:25Z",
        "patchSets": [
          1,
          2
        ],
        "result": "in progress",
        "rollingFrom": "f8a6b5b4b0d02895f70af4158c47c3069488a64a",
        "rollingTo": "082323b57da7c15e9d43ea3b68d2e6907e9b8139",
        "subject": "Roll Skia from f8a6b5b4b0d0 to 082323b57da7 (8 revisions)",
        "tryResults": [
          {
            "builder": "android-binary-size",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175571136"
          },
          {
            "builder": "android-lollipop-arm-rel",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175571120"
          },
          {
            "builder": "android-marshmallow-arm64-rel",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175571104"
          },
          {
            "builder": "android-pie-arm64-rel",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175571088"
          },
          {
            "builder": "android_compile_dbg",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175571072"
          },
          {
            "builder": "android_cronet",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175571056"
          },
          {
            "builder": "android_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175571040"
          },
          {
            "builder": "cast_shell_android",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175571024"
          },
          {
            "builder": "cast_shell_linux",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175571008"
          },
          {
            "builder": "chromeos-amd64-generic-rel",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570992"
          },
          {
            "builder": "chromeos-arm-generic-rel",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570976"
          },
          {
            "builder": "chromium_presubmit",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570960"
          },
          {
            "builder": "fuchsia-compile-x64-dbg",
            "category": "cq_experimental",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570944"
          },
          {
            "builder": "fuchsia-x64-cast",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570928"
          },
          {
            "builder": "fuchsia_arm64",
            "category": "cq_experimental",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570912"
          },
          {
            "builder": "fuchsia_x64",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570896"
          },
          {
            "builder": "ios-simulator",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570880"
          },
          {
            "builder": "linux-blink-rel",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570864"
          },
          {
            "builder": "linux-chromeos-compile-dbg",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570848"
          },
          {
            "builder": "linux-chromeos-rel",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570832"
          },
          {
            "builder": "linux-libfuzzer-asan-rel",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570816"
          },
          {
            "builder": "linux-ozone-rel",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570800"
          },
          {
            "builder": "linux-rel",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570784"
          },
          {
            "builder": "linux_chromium_asan_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570768"
          },
          {
            "builder": "linux_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570752"
          },
          {
            "builder": "linux_chromium_tsan_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570736"
          },
          {
            "builder": "linux_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570720"
          },
          {
            "builder": "mac-rel",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "",
            "status": "STARTED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570704"
          },
          {
            "builder": "mac_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570688"
          },
          {
            "builder": "mac_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570672"
          },
          {
            "builder": "win-libfuzzer-asan-rel",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570656"
          },
          {
            "builder": "win10_chromium_x64_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "",
            "status": "STARTED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570640"
          },
          {
            "builder": "win_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570624"
          },
          {
            "builder": "win_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-10T14:55:35.546417Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875151549175570608"
          }
        ]
      },
      {
        "closed": true,
        "comments": null,
        "committed": true,
        "cqFinished": true,
        "cqSuccess": true,
        "created": "2020-07-10T13:10:00Z",
        "dryRunFinished": false,
        "dryRunSuccess": false,
        "isDryRun": false,
        "issue": 2292355,
        "modified": "2020-07-10T14:52:11Z",
        "patchSets": [
          1,
          2,
          3
        ],
        "result": "succeeded",
        "rollingFrom": "9f821489c9f39c53bba496217cd7c1cf2ae9742b",
        "rollingTo": "f8a6b5b4b0d02895f70af4158c47c3069488a64a",
        "subject": "Roll Skia from 9f821489c9f3 to f8a6b5b4b0d0 (1 revision)",
        "tryResults": [
          {
            "builder": "android-binary-size",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557026080"
          },
          {
            "builder": "android-lollipop-arm-rel",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557026064"
          },
          {
            "builder": "android-marshmallow-arm64-rel",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557026048"
          },
          {
            "builder": "android-pie-arm64-rel",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557026032"
          },
          {
            "builder": "android_compile_dbg",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557026016"
          },
          {
            "builder": "android_cronet",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557026000"
          },
          {
            "builder": "android_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025984"
          },
          {
            "builder": "cast_shell_android",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025968"
          },
          {
            "builder": "cast_shell_linux",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025952"
          },
          {
            "builder": "chromeos-amd64-generic-rel",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025936"
          },
          {
            "builder": "chromeos-arm-generic-rel",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025920"
          },
          {
            "builder": "chromium_presubmit",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025904"
          },
          {
            "builder": "fuchsia-compile-x64-dbg",
            "category": "cq_experimental",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025888"
          },
          {
            "builder": "fuchsia-x64-cast",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025872"
          },
          {
            "builder": "fuchsia_arm64",
            "category": "cq_experimental",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025856"
          },
          {
            "builder": "fuchsia_x64",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025840"
          },
          {
            "builder": "ios-simulator",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025824"
          },
          {
            "builder": "linux-blink-rel",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025808"
          },
          {
            "builder": "linux-chromeos-compile-dbg",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025792"
          },
          {
            "builder": "linux-chromeos-rel",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025776"
          },
          {
            "builder": "linux-libfuzzer-asan-rel",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025760"
          },
          {
            "builder": "linux-ozone-rel",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025744"
          },
          {
            "builder": "linux-rel",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025728"
          },
          {
            "builder": "linux_chromium_asan_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025712"
          },
          {
            "builder": "linux_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025696"
          },
          {
            "builder": "linux_chromium_tsan_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025680"
          },
          {
            "builder": "linux_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025664"
          },
          {
            "builder": "mac-rel",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025648"
          },
          {
            "builder": "mac_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025632"
          },
          {
            "builder": "mac_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025616"
          },
          {
            "builder": "win-libfuzzer-asan-rel",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025600"
          },
          {
            "builder": "win10_chromium_x64_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025584"
          },
          {
            "builder": "win_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025568"
          },
          {
            "builder": "win_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-10T13:10:28.534039Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875158162557025552"
          }
        ]
      },
      {
        "closed": true,
        "comments": null,
        "committed": true,
        "cqFinished": true,
        "cqSuccess": true,
        "created": "2020-07-10T05:56:00Z",
        "dryRunFinished": false,
        "dryRunSuccess": false,
        "isDryRun": false,
        "issue": 2290799,
        "modified": "2020-07-10T07:33:35Z",
        "patchSets": [
          1,
          2,
          3
        ],
        "result": "succeeded",
        "rollingFrom": "4d48bb35972fdd94b27ce2739b9298a1267fe50e",
        "rollingTo": "9f821489c9f39c53bba496217cd7c1cf2ae9742b",
        "subject": "Roll Skia from 4d48bb35972f to 9f821489c9f3 (4 revisions)",
        "tryResults": [
          {
            "builder": "android-binary-size",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146944"
          },
          {
            "builder": "android-lollipop-arm-rel",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146928"
          },
          {
            "builder": "android-marshmallow-arm64-rel",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146912"
          },
          {
            "builder": "android-pie-arm64-rel",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146896"
          },
          {
            "builder": "android_compile_dbg",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146880"
          },
          {
            "builder": "android_cronet",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146864"
          },
          {
            "builder": "android_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146848"
          },
          {
            "builder": "cast_shell_android",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146832"
          },
          {
            "builder": "cast_shell_linux",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146816"
          },
          {
            "builder": "chromeos-amd64-generic-rel",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146800"
          },
          {
            "builder": "chromeos-arm-generic-rel",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146784"
          },
          {
            "builder": "chromium_presubmit",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146768"
          },
          {
            "builder": "fuchsia-compile-x64-dbg",
            "category": "cq_experimental",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146752"
          },
          {
            "builder": "fuchsia-x64-cast",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146736"
          },
          {
            "builder": "fuchsia_arm64",
            "category": "cq_experimental",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146720"
          },
          {
            "builder": "fuchsia_x64",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146704"
          },
          {
            "builder": "ios-simulator",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146688"
          },
          {
            "builder": "linux-blink-rel",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146672"
          },
          {
            "builder": "linux-chromeos-compile-dbg",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146656"
          },
          {
            "builder": "linux-chromeos-rel",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146640"
          },
          {
            "builder": "linux-libfuzzer-asan-rel",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146624"
          },
          {
            "builder": "linux-ozone-rel",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146608"
          },
          {
            "builder": "linux-rel",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146592"
          },
          {
            "builder": "linux_chromium_asan_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146576"
          },
          {
            "builder": "linux_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146560"
          },
          {
            "builder": "linux_chromium_tsan_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146544"
          },
          {
            "builder": "linux_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146528"
          },
          {
            "builder": "mac-rel",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146512"
          },
          {
            "builder": "mac_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146496"
          },
          {
            "builder": "mac_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146480"
          },
          {
            "builder": "win-libfuzzer-asan-rel",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146464"
          },
          {
            "builder": "win10_chromium_x64_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146448"
          },
          {
            "builder": "win_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146432"
          },
          {
            "builder": "win_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-10T05:56:37.900835Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875185457654146416"
          }
        ]
      },
      {
        "closed": true,
        "comments": null,
        "committed": true,
        "cqFinished": true,
        "cqSuccess": true,
        "created": "2020-07-10T04:41:00Z",
        "dryRunFinished": false,
        "dryRunSuccess": false,
        "isDryRun": false,
        "issue": 2291674,
        "modified": "2020-07-10T05:53:45Z",
        "patchSets": [
          1,
          2,
          3
        ],
        "result": "succeeded",
        "rollingFrom": "89d33d0a25b51b87b26e4683d6a889efc7f1968c",
        "rollingTo": "4d48bb35972fdd94b27ce2739b9298a1267fe50e",
        "subject": "Roll Skia from 89d33d0a25b5 to 4d48bb35972f (1 revision)",
        "tryResults": [
          {
            "builder": "android-binary-size",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551069168"
          },
          {
            "builder": "android-lollipop-arm-rel",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551069152"
          },
          {
            "builder": "android-marshmallow-arm64-rel",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551069136"
          },
          {
            "builder": "android-pie-arm64-rel",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551069120"
          },
          {
            "builder": "android_compile_dbg",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551069104"
          },
          {
            "builder": "android_cronet",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551069088"
          },
          {
            "builder": "android_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551069072"
          },
          {
            "builder": "cast_shell_android",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551069056"
          },
          {
            "builder": "cast_shell_linux",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551069040"
          },
          {
            "builder": "chromeos-amd64-generic-rel",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551069024"
          },
          {
            "builder": "chromeos-arm-generic-rel",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551069008"
          },
          {
            "builder": "chromium_presubmit",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551068992"
          },
          {
            "builder": "fuchsia-x64-cast",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551068976"
          },
          {
            "builder": "fuchsia_arm64",
            "category": "cq_experimental",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551068960"
          },
          {
            "builder": "fuchsia_x64",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551068944"
          },
          {
            "builder": "ios-simulator",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551068928"
          },
          {
            "builder": "linux-blink-rel",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551068912"
          },
          {
            "builder": "linux-chromeos-compile-dbg",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551068896"
          },
          {
            "builder": "linux-chromeos-rel",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551068880"
          },
          {
            "builder": "linux-libfuzzer-asan-rel",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551068864"
          },
          {
            "builder": "linux-ozone-rel",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551068848"
          },
          {
            "builder": "linux-rel",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551068832"
          },
          {
            "builder": "linux_chromium_asan_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551068816"
          },
          {
            "builder": "linux_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551068800"
          },
          {
            "builder": "linux_chromium_tsan_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551068784"
          },
          {
            "builder": "linux_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551068768"
          },
          {
            "builder": "mac-rel",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551068752"
          },
          {
            "builder": "mac_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551068736"
          },
          {
            "builder": "mac_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551068720"
          },
          {
            "builder": "win-libfuzzer-asan-rel",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551068704"
          },
          {
            "builder": "win10_chromium_x64_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551068688"
          },
          {
            "builder": "win_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551068672"
          },
          {
            "builder": "win_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-10T04:41:28.073846Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875190186551068656"
          }
        ]
      },
      {
        "closed": true,
        "comments": null,
        "committed": true,
        "cqFinished": true,
        "cqSuccess": true,
        "created": "2020-07-09T22:59:00Z",
        "dryRunFinished": false,
        "dryRunSuccess": false,
        "isDryRun": false,
        "issue": 2290836,
        "modified": "2020-07-10T01:03:08Z",
        "patchSets": [
          1,
          2,
          3
        ],
        "result": "succeeded",
        "rollingFrom": "fcfd0af9fd4e3a904530bcf63c6437ee93f347d8",
        "rollingTo": "89d33d0a25b51b87b26e4683d6a889efc7f1968c",
        "subject": "Roll Skia from fcfd0af9fd4e to 89d33d0a25b5 (11 revisions)",
        "tryResults": [
          {
            "builder": "android-binary-size",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160608"
          },
          {
            "builder": "android-lollipop-arm-rel",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160592"
          },
          {
            "builder": "android-marshmallow-arm64-rel",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160576"
          },
          {
            "builder": "android-pie-arm64-rel",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160560"
          },
          {
            "builder": "android_compile_dbg",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160544"
          },
          {
            "builder": "android_cronet",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160528"
          },
          {
            "builder": "android_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160512"
          },
          {
            "builder": "cast_shell_android",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160496"
          },
          {
            "builder": "cast_shell_linux",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160480"
          },
          {
            "builder": "chromeos-amd64-generic-rel",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160464"
          },
          {
            "builder": "chromeos-arm-generic-rel",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160448"
          },
          {
            "builder": "chromium_presubmit",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160432"
          },
          {
            "builder": "fuchsia-x64-cast",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160416"
          },
          {
            "builder": "fuchsia_arm64",
            "category": "cq_experimental",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160400"
          },
          {
            "builder": "fuchsia_x64",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160384"
          },
          {
            "builder": "ios-simulator",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160368"
          },
          {
            "builder": "linux-blink-rel",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160352"
          },
          {
            "builder": "linux-chromeos-compile-dbg",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160336"
          },
          {
            "builder": "linux-chromeos-rel",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160320"
          },
          {
            "builder": "linux-libfuzzer-asan-rel",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160304"
          },
          {
            "builder": "linux-ozone-rel",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160288"
          },
          {
            "builder": "linux-rel",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160272"
          },
          {
            "builder": "linux_chromium_asan_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160256"
          },
          {
            "builder": "linux_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160240"
          },
          {
            "builder": "linux_chromium_tsan_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160224"
          },
          {
            "builder": "linux_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160208"
          },
          {
            "builder": "mac-rel",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160192"
          },
          {
            "builder": "mac_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160176"
          },
          {
            "builder": "mac_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160160"
          },
          {
            "builder": "win-libfuzzer-asan-rel",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160144"
          },
          {
            "builder": "win10_chromium_x64_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160128"
          },
          {
            "builder": "win_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160112"
          },
          {
            "builder": "win_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T22:59:33.435607Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875211697708160096"
          }
        ]
      },
      {
        "closed": true,
        "comments": null,
        "committed": true,
        "cqFinished": true,
        "cqSuccess": true,
        "created": "2020-07-09T19:28:00Z",
        "dryRunFinished": false,
        "dryRunSuccess": false,
        "isDryRun": false,
        "issue": 2290315,
        "modified": "2020-07-09T22:56:04Z",
        "patchSets": [
          1,
          2,
          3
        ],
        "result": "succeeded",
        "rollingFrom": "9e8f484499bd04b8112f845589e9a7d4eec90791",
        "rollingTo": "fcfd0af9fd4e3a904530bcf63c6437ee93f347d8",
        "subject": "Roll Skia from 9e8f484499bd to fcfd0af9fd4e (10 revisions)",
        "tryResults": [
          {
            "builder": "android-binary-size",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675488"
          },
          {
            "builder": "android-lollipop-arm-rel",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675472"
          },
          {
            "builder": "android-marshmallow-arm64-rel",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675456"
          },
          {
            "builder": "android-pie-arm64-rel",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675440"
          },
          {
            "builder": "android_compile_dbg",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675424"
          },
          {
            "builder": "android_cronet",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675408"
          },
          {
            "builder": "android_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675392"
          },
          {
            "builder": "cast_shell_android",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675376"
          },
          {
            "builder": "cast_shell_linux",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675360"
          },
          {
            "builder": "chromeos-amd64-generic-rel",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675344"
          },
          {
            "builder": "chromeos-arm-generic-rel",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675328"
          },
          {
            "builder": "chromium_presubmit",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675312"
          },
          {
            "builder": "fuchsia-compile-x64-dbg",
            "category": "cq_experimental",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675296"
          },
          {
            "builder": "fuchsia-x64-cast",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675280"
          },
          {
            "builder": "fuchsia_arm64",
            "category": "cq_experimental",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675264"
          },
          {
            "builder": "fuchsia_x64",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675248"
          },
          {
            "builder": "ios-simulator",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675232"
          },
          {
            "builder": "linux-blink-rel",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675216"
          },
          {
            "builder": "linux-chromeos-compile-dbg",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675200"
          },
          {
            "builder": "linux-chromeos-rel",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675184"
          },
          {
            "builder": "linux-libfuzzer-asan-rel",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675168"
          },
          {
            "builder": "linux-ozone-rel",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675152"
          },
          {
            "builder": "linux-rel",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675136"
          },
          {
            "builder": "linux_chromium_asan_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675120"
          },
          {
            "builder": "linux_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675104"
          },
          {
            "builder": "linux_chromium_tsan_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675088"
          },
          {
            "builder": "linux_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675072"
          },
          {
            "builder": "mac-rel",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675056"
          },
          {
            "builder": "mac_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675040"
          },
          {
            "builder": "mac_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675024"
          },
          {
            "builder": "win-libfuzzer-asan-rel",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605675008"
          },
          {
            "builder": "win10_chromium_x64_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "FAILURE",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605674992"
          },
          {
            "builder": "win10_chromium_x64_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-09T21:20:26.94952Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875217933050775744"
          },
          {
            "builder": "win_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605674976"
          },
          {
            "builder": "win_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T19:28:43.043006Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875224962605674960"
          }
        ]
      },
      {
        "closed": true,
        "comments": null,
        "committed": true,
        "cqFinished": true,
        "cqSuccess": true,
        "created": "2020-07-09T17:01:00Z",
        "dryRunFinished": false,
        "dryRunSuccess": false,
        "isDryRun": false,
        "issue": 2289905,
        "modified": "2020-07-09T19:25:17Z",
        "patchSets": [
          1,
          2,
          3
        ],
        "result": "succeeded",
        "rollingFrom": "196515cbd749159ccc77980da4d041815924f8ce",
        "rollingTo": "9e8f484499bd04b8112f845589e9a7d4eec90791",
        "subject": "Roll Skia from 196515cbd749 to 9e8f484499bd (5 revisions)",
        "tryResults": [
          {
            "builder": "android-binary-size",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144624"
          },
          {
            "builder": "android-lollipop-arm-rel",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144608"
          },
          {
            "builder": "android-marshmallow-arm64-rel",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144592"
          },
          {
            "builder": "android-pie-arm64-rel",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144576"
          },
          {
            "builder": "android_compile_dbg",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144560"
          },
          {
            "builder": "android_cronet",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144544"
          },
          {
            "builder": "android_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144528"
          },
          {
            "builder": "cast_shell_android",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144512"
          },
          {
            "builder": "cast_shell_linux",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144496"
          },
          {
            "builder": "chromeos-amd64-generic-rel",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144480"
          },
          {
            "builder": "chromeos-arm-generic-rel",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144464"
          },
          {
            "builder": "chromium_presubmit",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144448"
          },
          {
            "builder": "fuchsia-compile-x64-dbg",
            "category": "cq_experimental",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144432"
          },
          {
            "builder": "fuchsia-x64-cast",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144416"
          },
          {
            "builder": "fuchsia_arm64",
            "category": "cq_experimental",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144400"
          },
          {
            "builder": "fuchsia_x64",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144384"
          },
          {
            "builder": "ios-simulator",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144368"
          },
          {
            "builder": "linux-blink-rel",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144352"
          },
          {
            "builder": "linux-chromeos-compile-dbg",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144336"
          },
          {
            "builder": "linux-chromeos-rel",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144320"
          },
          {
            "builder": "linux-libfuzzer-asan-rel",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144304"
          },
          {
            "builder": "linux-ozone-rel",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144288"
          },
          {
            "builder": "linux-rel",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144272"
          },
          {
            "builder": "linux_chromium_asan_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144256"
          },
          {
            "builder": "linux_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144240"
          },
          {
            "builder": "linux_chromium_tsan_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144224"
          },
          {
            "builder": "linux_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144208"
          },
          {
            "builder": "mac-rel",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144192"
          },
          {
            "builder": "mac_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144176"
          },
          {
            "builder": "mac_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144160"
          },
          {
            "builder": "win-libfuzzer-asan-rel",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144144"
          },
          {
            "builder": "win10_chromium_x64_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144128"
          },
          {
            "builder": "win_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144112"
          },
          {
            "builder": "win_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T17:01:36.671769Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875234217727144096"
          }
        ]
      },
      {
        "closed": true,
        "comments": null,
        "committed": true,
        "cqFinished": true,
        "cqSuccess": true,
        "created": "2020-07-09T15:08:00Z",
        "dryRunFinished": false,
        "dryRunSuccess": false,
        "isDryRun": false,
        "issue": 2289894,
        "modified": "2020-07-09T16:58:34Z",
        "patchSets": [
          1,
          2,
          3
        ],
        "result": "succeeded",
        "rollingFrom": "6669b01267057f32bd02cddad00b8ad3cbbac918",
        "rollingTo": "196515cbd749159ccc77980da4d041815924f8ce",
        "subject": "Roll Skia from 6669b0126705 to 196515cbd749 (11 revisions)",
        "tryResults": [
          {
            "builder": "android-binary-size",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854768"
          },
          {
            "builder": "android-lollipop-arm-rel",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854752"
          },
          {
            "builder": "android-marshmallow-arm64-rel",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854736"
          },
          {
            "builder": "android-pie-arm64-rel",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854720"
          },
          {
            "builder": "android_compile_dbg",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854704"
          },
          {
            "builder": "android_cronet",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854688"
          },
          {
            "builder": "android_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854672"
          },
          {
            "builder": "cast_shell_android",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854656"
          },
          {
            "builder": "cast_shell_linux",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854640"
          },
          {
            "builder": "chromeos-amd64-generic-rel",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854624"
          },
          {
            "builder": "chromeos-arm-generic-rel",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854608"
          },
          {
            "builder": "chromium_presubmit",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854592"
          },
          {
            "builder": "fuchsia-x64-cast",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854576"
          },
          {
            "builder": "fuchsia_arm64",
            "category": "cq_experimental",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854560"
          },
          {
            "builder": "fuchsia_x64",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854544"
          },
          {
            "builder": "ios-simulator",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854528"
          },
          {
            "builder": "linux-blink-rel",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854512"
          },
          {
            "builder": "linux-chromeos-compile-dbg",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854496"
          },
          {
            "builder": "linux-chromeos-rel",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854480"
          },
          {
            "builder": "linux-libfuzzer-asan-rel",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854464"
          },
          {
            "builder": "linux-ozone-rel",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854448"
          },
          {
            "builder": "linux-rel",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854432"
          },
          {
            "builder": "linux_chromium_asan_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854416"
          },
          {
            "builder": "linux_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854400"
          },
          {
            "builder": "linux_chromium_tsan_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854384"
          },
          {
            "builder": "linux_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854368"
          },
          {
            "builder": "mac-rel",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854352"
          },
          {
            "builder": "mac_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854336"
          },
          {
            "builder": "mac_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854320"
          },
          {
            "builder": "win-libfuzzer-asan-rel",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854304"
          },
          {
            "builder": "win10_chromium_x64_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854288"
          },
          {
            "builder": "win_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854272"
          },
          {
            "builder": "win_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T15:08:33.306784Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875241330600854256"
          }
        ]
      },
      {
        "closed": true,
        "comments": null,
        "committed": false,
        "cqFinished": true,
        "cqSuccess": false,
        "created": "2020-07-09T13:58:00Z",
        "dryRunFinished": false,
        "dryRunSuccess": false,
        "isDryRun": false,
        "issue": 2289808,
        "modified": "2020-07-09T15:07:00Z",
        "patchSets": [
          1,
          2
        ],
        "result": "failed",
        "rollingFrom": "6669b01267057f32bd02cddad00b8ad3cbbac918",
        "rollingTo": "dadc0819e9dcdfd11092c19d3a00fb67f5f3d754",
        "subject": "Roll Skia from 6669b0126705 to dadc0819e9dc (8 revisions)",
        "tryResults": [
          {
            "builder": "android-binary-size",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107828208"
          },
          {
            "builder": "android-lollipop-arm-rel",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107828192"
          },
          {
            "builder": "android-marshmallow-arm64-rel",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107828176"
          },
          {
            "builder": "android-pie-arm64-rel",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107828160"
          },
          {
            "builder": "android_compile_dbg",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107828144"
          },
          {
            "builder": "android_cronet",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107828128"
          },
          {
            "builder": "android_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107828112"
          },
          {
            "builder": "cast_shell_android",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107828096"
          },
          {
            "builder": "cast_shell_linux",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107828080"
          },
          {
            "builder": "chromeos-amd64-generic-rel",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107828064"
          },
          {
            "builder": "chromeos-arm-generic-rel",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107828048"
          },
          {
            "builder": "chromium_presubmit",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107828032"
          },
          {
            "builder": "fuchsia-x64-cast",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107828016"
          },
          {
            "builder": "fuchsia_arm64",
            "category": "cq_experimental",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107828000"
          },
          {
            "builder": "fuchsia_x64",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107827984"
          },
          {
            "builder": "ios-simulator",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "",
            "status": "STARTED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107827968"
          },
          {
            "builder": "linux-blink-rel",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107827952"
          },
          {
            "builder": "linux-chromeos-compile-dbg",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107827936"
          },
          {
            "builder": "linux-chromeos-rel",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107827920"
          },
          {
            "builder": "linux-libfuzzer-asan-rel",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107827904"
          },
          {
            "builder": "linux-ozone-rel",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107827888"
          },
          {
            "builder": "linux-rel",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "",
            "status": "STARTED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107827872"
          },
          {
            "builder": "linux_chromium_asan_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107827856"
          },
          {
            "builder": "linux_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107827840"
          },
          {
            "builder": "linux_chromium_tsan_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107827824"
          },
          {
            "builder": "linux_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107827808"
          },
          {
            "builder": "mac-rel",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "",
            "status": "STARTED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107827792"
          },
          {
            "builder": "mac_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107827776"
          },
          {
            "builder": "mac_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "FAILURE",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107827760"
          },
          {
            "builder": "mac_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T14:39:15.204335Z",
            "result": "FAILURE",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875243174104967040"
          },
          {
            "builder": "win-libfuzzer-asan-rel",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107827744"
          },
          {
            "builder": "win10_chromium_x64_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "",
            "status": "STARTED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107827728"
          },
          {
            "builder": "win_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107827712"
          },
          {
            "builder": "win_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T13:58:28.07345Z",
            "result": "FAILURE",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875245740107827696"
          },
          {
            "builder": "win_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T14:47:47.295939Z",
            "result": "",
            "status": "STARTED",
            "url": "https://cr-buildbucket.appspot.com/build/8875242637138246528"
          }
        ]
      },
      {
        "closed": true,
        "comments": null,
        "committed": false,
        "cqFinished": true,
        "cqSuccess": false,
        "created": "2020-07-09T12:49:00Z",
        "dryRunFinished": false,
        "dryRunSuccess": false,
        "isDryRun": false,
        "issue": 2289804,
        "modified": "2020-07-09T13:57:00Z",
        "patchSets": [
          1,
          2
        ],
        "result": "failed",
        "rollingFrom": "6669b01267057f32bd02cddad00b8ad3cbbac918",
        "rollingTo": "16bf7d31c8192c37bba1c7451ac578e73d2350ca",
        "subject": "Roll Skia from 6669b0126705 to 16bf7d31c819 (6 revisions)",
        "tryResults": [
          {
            "builder": "android-binary-size",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233760"
          },
          {
            "builder": "android-lollipop-arm-rel",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233744"
          },
          {
            "builder": "android-marshmallow-arm64-rel",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233728"
          },
          {
            "builder": "android-pie-arm64-rel",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233712"
          },
          {
            "builder": "android_compile_dbg",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233696"
          },
          {
            "builder": "android_cronet",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233680"
          },
          {
            "builder": "android_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233664"
          },
          {
            "builder": "cast_shell_android",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233648"
          },
          {
            "builder": "cast_shell_linux",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233632"
          },
          {
            "builder": "chromeos-amd64-generic-rel",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233616"
          },
          {
            "builder": "chromeos-arm-generic-rel",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233600"
          },
          {
            "builder": "chromium_presubmit",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233584"
          },
          {
            "builder": "fuchsia-x64-cast",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233568"
          },
          {
            "builder": "fuchsia_arm64",
            "category": "cq_experimental",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233552"
          },
          {
            "builder": "fuchsia_x64",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233536"
          },
          {
            "builder": "ios-simulator",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "",
            "status": "STARTED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233520"
          },
          {
            "builder": "linux-blink-rel",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233504"
          },
          {
            "builder": "linux-chromeos-compile-dbg",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233488"
          },
          {
            "builder": "linux-chromeos-rel",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "",
            "status": "STARTED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233472"
          },
          {
            "builder": "linux-libfuzzer-asan-rel",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233456"
          },
          {
            "builder": "linux-ozone-rel",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233440"
          },
          {
            "builder": "linux-rel",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "",
            "status": "STARTED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233424"
          },
          {
            "builder": "linux_chromium_asan_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233408"
          },
          {
            "builder": "linux_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233392"
          },
          {
            "builder": "linux_chromium_tsan_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233376"
          },
          {
            "builder": "linux_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233360"
          },
          {
            "builder": "mac-rel",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "",
            "status": "STARTED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233344"
          },
          {
            "builder": "mac_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233328"
          },
          {
            "builder": "mac_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "FAILURE",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233312"
          },
          {
            "builder": "mac_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T13:28:02.875789Z",
            "result": "FAILURE",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875247653966540976"
          },
          {
            "builder": "win-libfuzzer-asan-rel",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233296"
          },
          {
            "builder": "win10_chromium_x64_rel_ng",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "",
            "status": "STARTED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233280"
          },
          {
            "builder": "win_chromium_compile_dbg_ng",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "SUCCESS",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233264"
          },
          {
            "builder": "win_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T12:49:32.252487Z",
            "result": "FAILURE",
            "status": "COMPLETED",
            "url": "https://cr-buildbucket.appspot.com/build/8875250076830233248"
          },
          {
            "builder": "win_optional_gpu_tests_rel",
            "category": "cq",
            "created_ts": "2020-07-09T13:42:49.384219Z",
            "result": "",
            "status": "STARTED",
            "url": "https://cr-buildbucket.appspot.com/build/8875246724394552784"
          }
        ]
      }
    ],
    "status": "active",
    "strategy": {
      "message": "going back to normal, doing manual rolls instead",
      "strategy": "batch",
      "time": "2020-06-24T18:51:37.308835Z",
      "user": "egdaniel@google.com"
    },
    "throttledUntil": -62135596800,
    "validModes": [
      "running",
      "stopped",
      "dry run"
    ],
    "validStrategies": [
      "batch",
      "single"
    ]
  }
  `);