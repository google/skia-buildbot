export const singleResultCanDelete = {
  data: [
    {
      DatastoreKey: 'ChMiEWNsdXN0ZXItdGVsZW1ldHJ5EhYKEUNocm9taXVtUGVyZlRhc2tzEPUk',
      TsAdded: 20200310143034,
      TsStarted: 20200310143040,
      TsCompleted: 0,
      Username: 'user@example.com',
      Failure: false,
      RepeatAfterDays: 2,
      SwarmingLogs:
        'https://chrome-swarming.appspot.com/tasklist?l=500&c=name&c=created_ts&c=bot&c=duration&c=state&f=runid:rmistry-ChromiumPerf-4725&st=1262304000000',
      TaskDone: false,
      SwarmingTaskID: '4addac5a188dbc10',
      RepeatRuns: 1,
    },
  ],
  ids: [1],
  pagination: {
    offset: 0,
    size: 1,
    total: 2,
  },
  permissions: [
    {
      DeleteAllowed: true,
      RedoAllowed: false,
    },
  ],
};

export const singleResultNoDelete = {
  data: [
    {
      DatastoreKey: 'ChMiEWNsdXN0ZXItdGVsZW1ldHJ5EhYKEUNocm9taXVtUGVyZlRhc2tzEPUk',
      TsAdded: 20200310143034,
      TsStarted: 20200310143040,
      TsCompleted: 0,
      Username: 'user@example.com',
      Failure: false,
      RepeatAfterDays: 2,
      SwarmingLogs:
        'https://chrome-swarming.appspot.com/tasklist?l=500&c=name&c=created_ts&c=bot&c=duration&c=state&f=runid:rmistry-ChromiumPerf-4725&st=1262304000000',
      TaskDone: false,
      SwarmingTaskID: '4addac5a188dbc10',
      RepeatRuns: 1,
    },
  ],
  ids: [2],
  pagination: {
    offset: 0,
    size: 1,
    total: 2,
  },
  permissions: [
    {
      DeleteAllowed: false,
      RedoAllowed: false,
    },
  ],
};

export const resultSetOneItem = singleResultNoDelete;

export const resultSetTwoItems = {
  data: [
    {
      DatastoreKey: 'ChMiEWNsdXN0ZXItdGVsZW1ldHJ5EhoKFUNocm9taXVtQW5hbHlzaXNUYXNrcxD7Dw',
      TsAdded: 20200309185034,
      TsStarted: 20200309185121,
      TsCompleted: 20200309203134,
      Username: 'user@example.com',
      Failure: false,
      RepeatAfterDays: 1,
      SwarmingLogs:
        'https://chrome-swarming.appspot.com/tasklist?l=500&c=name&c=created_ts&c=bot&c=duration&c=state&f=runid:alexmt-ChromiumAnalysis-2043&st=1262304000000',
      TaskDone: true,
      SwarmingTaskID: '4ad974a75652da10',
      Benchmark: 'ad_tagging.cluster_telemetry',
      PageSets: '10k',
      IsTestPageSet: false,
      BenchmarkArgs:
        '--output-format=csv --skip-typ-expectations-tags-validation --legacy-json-trace-format',
      BrowserArgs: '',
      Description: 'Regular AdTagging accuracy run',
      CustomWebpagesGSPath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      ChromiumPatchGSPath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      SkiaPatchGSPath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      CatapultPatchGSPath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      BenchmarkPatchGSPath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      V8PatchGSPath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      RunInParallel: false,
      Platform: 'Linux',
      RunOnGCE: false,
      RawOutput:
        'https://ct.skia.org/results/cluster-telemetry/tasks/benchmark_runs/alexmt-ChromiumAnalysis-2043/consolidated_outputs/alexmt-ChromiumAnalysis-2043.output',
      ValueColumnName: 'sum',
      MatchStdoutTxt: '',
      ChromiumHash: '',
      ApkGsPath: '',
      TelemetryIsolateHash: '',
      CCList: null,
      TaskPriority: 110,
      GroupName: 'AdTagging',
    },
    {
      DatastoreKey: 'ChMiEWNsdXN0ZXItdGVsZW1ldHJ5EhoKFUNocm9taXVtQW5hbHlzaXNUYXNrcxD5Dw',
      TsAdded: 20200308152034,
      TsStarted: 20200308152039,
      TsCompleted: 20200309010734,
      Username: 'someoneWithAReallyLongUsernameJustToSeeHowItGoes@example.com',
      Failure: false,
      RepeatAfterDays: 2,
      SwarmingLogs:
        'https://chrome-swarming.appspot.com/tasklist?l=500&c=name&c=created_ts&c=bot&c=duration&c=state&f=runid:dproy-ChromiumAnalysis-2041&st=1262304000000',
      TaskDone: true,
      SwarmingTaskID: '4ad38d6496eef010',
      Benchmark: 'loading.cluster_telemetry',
      PageSets: 'VoltMobile10k',
      IsTestPageSet: false,
      BenchmarkArgs:
        '--output-format=csv --pageset-repeat=1 --skip-typ-expectations-tags-validation --legacy-json-trace-format --traffic-setting=Regular-4G --use-live-sites',
      BrowserArgs: '',
      Description: 'Regular run for Volt 10k pages (Chrome M80, live sites) ',
      CustomWebpagesGSPath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      ChromiumPatchGSPath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      SkiaPatchGSPath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      CatapultPatchGSPath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      BenchmarkPatchGSPath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      V8PatchGSPath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      RunInParallel: false,
      Platform: 'Android',
      RunOnGCE: false,
      RawOutput:
        'https://ct.skia.org/results/cluster-telemetry/tasks/benchmark_runs/dproy-ChromiumAnalysis-2041/consolidated_outputs/dproy-ChromiumAnalysis-2041.output',
      ValueColumnName: 'avg',
      MatchStdoutTxt: '',
      ChromiumHash: '',
      ApkGsPath: 'gs://chrome-unsigned/android-B0urB0N/80.0.3987.87/arm_64/ChromeModern.apk',
      TelemetryIsolateHash: 'b13b9f6b50847aab5f395c5be7ee1d71e2e7abd3',
      CCList: null,
      TaskPriority: 100,
      GroupName: 'volt10k-m80',
    },
  ],
  ids: [3, 4],
  pagination: {
    offset: 0,
    size: 10,
    total: 2,
  },
  permissions: [
    {
      DeleteAllowed: false,
      RedoAllowed: true,
    },
    {
      DeleteAllowed: false,
      RedoAllowed: true,
    },
  ],
};
