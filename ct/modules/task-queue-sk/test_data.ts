import {
  GetTasksResponse,
} from '../json';

export const singleResultCanDelete: GetTasksResponse = {
  data: [
    {
      ts_added: 20200310143034,
      ts_started: 20200310143040,
      ts_completed: 0,
      username: 'user@example.com',
      failure: false,
      repeat_after_days: 2,
      swarming_logs: 'https://chrome-swarming.appspot.com/tasklist?l=500&c=name&c=created_ts&c=bot&c=duration&c=state&f=runid:rmistry-ChromiumPerf-4725&st=1262304000000',
      task_done: false,
      swarming_task_id: '4addac5a188dbc10',
      repeat_runs: 1,
    },
  ],
  ids: [
    1,
  ],
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

export const singleResultNoDelete: GetTasksResponse = {
  data: [
    {
      ts_added: 20200310143034,
      ts_started: 20200310143040,
      ts_completed: 0,
      username: 'user@example.com',
      failure: false,
      repeat_after_days: 2,
      swarming_logs: 'https://chrome-swarming.appspot.com/tasklist?l=500&c=name&c=created_ts&c=bot&c=duration&c=state&f=runid:rmistry-ChromiumPerf-4725&st=1262304000000',
      task_done: false,
      swarming_task_id: '4addac5a188dbc10',
      repeat_runs: 1,
    },
  ],
  ids: [
    2,
  ],
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

export const resultSetOneItem: GetTasksResponse = singleResultNoDelete;

export const resultSetTwoItems: GetTasksResponse = {
  data: [
    {
      ts_added: 20200309185034,
      ts_started: 20200309185121,
      ts_completed: 20200309203134,
      username: 'user@example.com',
      failure: false,
      repeat_after_days: 1,
      swarming_logs: 'https://chrome-swarming.appspot.com/tasklist?l=500&c=name&c=created_ts&c=bot&c=duration&c=state&f=runid:alexmt-ChromiumAnalysis-2043&st=1262304000000',
      task_done: true,
      swarming_task_id: '4ad974a75652da10',
      benchmark: 'ad_tagging.cluster_telemetry',
      page_sets: '10k',
      is_test_page_set: false,
      benchmark_args: '--output-format=csv --skip-typ-expectations-tags-validation --legacy-json-trace-format',
      browser_args: '',
      description: 'Regular AdTagging accuracy run',
      custom_webpages_gspath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      chromium_patch_gspath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      skia_patch_gspath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      catapult_patch_gspath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      benchmark_patch_gspath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      v8_patch_gspath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      run_in_parallel: false,
      platform: 'Linux',
      run_on_gce: false,
      raw_output: 'https://ct.skia.org/results/cluster-telemetry/tasks/benchmark_runs/alexmt-ChromiumAnalysis-2043/consolidated_outputs/alexmt-ChromiumAnalysis-2043.output',
      value_column_name: 'sum',
      match_stdout_txt: '',
      chromium_hash: '',
      apk_gspath: '',
      telemetry_isolate_hash: '',
      cc_list: null,
      task_priority: 110,
      group_name: 'AdTagging',
    },
    {
      ts_added: 20200308152034,
      ts_started: 20200308152039,
      ts_completed: 20200309010734,
      username: 'someoneWithAReallyLongusernameJustToSeeHowItGoes@example.com',
      failure: false,
      repeat_after_days: 2,
      swarming_logs: 'https://chrome-swarming.appspot.com/tasklist?l=500&c=name&c=created_ts&c=bot&c=duration&c=state&f=runid:dproy-ChromiumAnalysis-2041&st=1262304000000',
      task_done: true,
      swarming_task_id: '4ad38d6496eef010',
      benchmark: 'loading.cluster_telemetry',
      page_sets: 'VoltMobile10k',
      is_test_page_set: false,
      benchmark_args: '--output-format=csv --pageset-repeat=1 --skip-typ-expectations-tags-validation --legacy-json-trace-format --traffic-setting=Regular-4G --use-live-sites',
      browser_args: '',
      description: 'Regular run for Volt 10k pages (Chrome M80, live sites) ',
      custom_webpages_gspath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      chromium_patch_gspath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      skia_patch_gspath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      catapult_patch_gspath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      benchmark_patch_gspath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      v8_patch_gspath: 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch',
      run_in_parallel: false,
      platform: 'Android',
      run_on_gce: false,
      raw_output: 'https://ct.skia.org/results/cluster-telemetry/tasks/benchmark_runs/dproy-ChromiumAnalysis-2041/consolidated_outputs/dproy-ChromiumAnalysis-2041.output',
      value_column_name: 'avg',
      match_stdout_txt: '',
      chromium_hash: '',
      apk_gspath: 'gs://chrome-unsigned/android-B0urB0N/80.0.3987.87/arm_64/ChromeModern.apk',
      telemetry_isolate_hash: 'b13b9f6b50847aab5f395c5be7ee1d71e2e7abd3',
      cc_list: null,
      task_priority: 100,
      group_name: 'volt10k-m80',
    },
  ],
  ids: [
    3,
    4,
  ],
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
