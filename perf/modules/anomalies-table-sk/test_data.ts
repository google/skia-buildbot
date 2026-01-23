import { Anomaly, BugType, GetGroupReportResponse, GraphConfig } from '../json';

export const anomaly_table = [
  {
    id: '1',
    test_path:
      'ChromiumPerf/mac-m1_mini_2020-perf/jetstream2/stanford-crypto-aes.Average/JetStream2',
    bug_id: 12345,
    start_revision: 1729647389,
    end_revision: 1739647389,
    is_improvement: false,
    recovered: true,
    state: '',
    statistic: '',
    units: 'ms',
    degrees_of_freedom: 0,
    median_before_anomaly: 75.209091,
    median_after_anomaly: 100.5023,
    p_value: 0,
    segment_size_after: 0,
    segment_size_before: 0,
    std_dev_before_anomaly: 0,
    t_statistic: 0,
    subscription_name: '',
    bug_component: 'Component C',
    bug_labels: [],
    bug_cc_emails: [],
    bisect_ids: [],
  },
  {
    id: '2',
    test_path: 'ChromiumPerf/win-10-perf/jetstream2/stanford-crypto-aes.Average/JetStream2',
    bug_id: 23456,
    start_revision: 1749788389,
    end_revision: 1759747389,
    is_improvement: false,
    recovered: true,
    state: '',
    statistic: '',
    units: 'delta',
    degrees_of_freedom: 0,
    median_before_anomaly: 75.209091,
    median_after_anomaly: 100.5023,
    p_value: 0,
    segment_size_after: 0,
    segment_size_before: 0,
    std_dev_before_anomaly: 0,
    t_statistic: 0,
    subscription_name: '',
    bug_component: 'Component A',
    bug_labels: [],
    bug_cc_emails: [],
    bisect_ids: [],
  },
  {
    id: '3',
    test_path: 'ChromiumPerf/mac-m1-pro-perf/jetstream2/stanford-crypto-aes.Average/JetStream2',
    bug_id: 34567,
    start_revision: 1749787389,
    end_revision: 1759777389,
    is_improvement: false,
    recovered: true,
    state: '',
    statistic: '',
    units: 'ms',
    degrees_of_freedom: 0,
    median_before_anomaly: 1.345,
    median_after_anomaly: 2.403,
    p_value: 0,
    segment_size_after: 0,
    segment_size_before: 0,
    std_dev_before_anomaly: 0,
    t_statistic: 0,
    subscription_name: '',
    bug_component: 'Component B',
    bug_labels: [],
    bug_cc_emails: [],
    bisect_ids: [],
  },
  {
    id: '4',
    test_path: 'ChromiumPerf/win-11-perf/jetstream2/stanford-crypto-aes.Average/JetStream2',
    bug_id: 12345,
    start_revision: 1749747389,
    end_revision: 1759747389,
    is_improvement: true,
    recovered: true,
    state: '',
    statistic: '',
    units: 'ns',
    degrees_of_freedom: 0,
    median_before_anomaly: 72.209091,
    median_after_anomaly: 73.5023,
    p_value: 0,
    segment_size_after: 0,
    segment_size_before: 0,
    std_dev_before_anomaly: 0,
    t_statistic: 0,
    subscription_name: '',
    bug_component: 'Component B',
    bug_labels: [],
    bug_cc_emails: [],
    bisect_ids: [],
  },
  {
    id: '5',
    test_path: 'ChromiumPerf/linux-perf-pgo/jetstream2/stanford-crypto-aes.Average/JetStream2',
    bug_id: -1,
    start_revision: 1849647389,
    end_revision: 1850947244,
    is_improvement: false,
    recovered: true,
    state: '',
    statistic: '',
    units: 'ms',
    degrees_of_freedom: 0,
    median_before_anomaly: 75.209091,
    median_after_anomaly: 100.5023,
    p_value: 0,
    segment_size_after: 0,
    segment_size_before: 0,
    std_dev_before_anomaly: 0,
    t_statistic: 0,
    subscription_name: '',
    bug_component: 'Component C',
    bug_labels: [],
    bug_cc_emails: [],
    bisect_ids: [],
  },
];

export const GROUP_REPORT_RESPONSE_WITH_SID: GetGroupReportResponse = {
  sid: 'test-sid',
  anomaly_list: anomaly_table,
  timerange_map: {
    1: { begin: 1719220000, end: 1719223600 },
    2: { begin: 1719227200, end: 1719230800 },
  },
  selected_keys: null,
  error: '',
  is_commit_number_based: true,
};

export const GROUP_REPORT_RESPONSE: GetGroupReportResponse = {
  sid: '',
  anomaly_list: anomaly_table,
  timerange_map: {
    1: { begin: 1719220000, end: 1719223600 },
    2: { begin: 1719227200, end: 1719230800 },
  },
  selected_keys: null,
  error: '',
  is_commit_number_based: true,
};

export const graphConfig: GraphConfig[] = [
  {
    keys: '',
    formulas: [],
    queries: ['config=8888&arch=x86'],
  },
];

const BASE_ANOMALY = {
  recovered: true,
  state: '',
  statistic: '',
  degrees_of_freedom: 0,
  p_value: 0,
  segment_size_after: 0,
  segment_size_before: 0,
  std_dev_before_anomaly: 0,
  t_statistic: 0,
  subscription_name: '',
  bug_labels: [],
  bug_cc_emails: [],
  bisect_ids: [],
  units: 'ms',
  is_improvement: false,
  median_before_anomaly: 75,
  median_after_anomaly: 100,
  bug_component: 'Test>Component',
};

export const anomaly_table_for_grouping = [
  // --- BUG ID GROUP (Always together) ---
  {
    ...BASE_ANOMALY,
    id: 'bug-1',
    bug_id: 12345,
    test_path: 'Master/BotA/BenchX/Test1/Sub',
    start_revision: 100,
    end_revision: 200, // Range doesn't matter
  },
  {
    ...BASE_ANOMALY,
    id: 'bug-2',
    bug_id: 12345,
    test_path: 'Master/BotB/BenchY/Test2/Sub', // Different attributes
    start_revision: 800,
    end_revision: 900, // Different range
  },

  // --- REVISION GROUP A: Exact Match (100-200) ---
  {
    ...BASE_ANOMALY,
    id: 'rev-a-1',
    bug_id: 0,
    test_path: 'Master/BotA/BenchX/Test1/Sub',
    start_revision: 100,
    end_revision: 200,
  },
  {
    ...BASE_ANOMALY,
    id: 'rev-a-2', // Identical to a-1
    bug_id: 0,
    test_path: 'Master/BotA/BenchX/Test1/Sub',
    start_revision: 100,
    end_revision: 200,
  },
  {
    ...BASE_ANOMALY,
    id: 'rev-a-3', // Same Rev, Different Bot
    bug_id: 0,
    test_path: 'Master/BotB/BenchX/Test1/Sub',
    start_revision: 100,
    end_revision: 200,
  },

  // --- REVISION GROUP B: Overlapping with A (150-250) ---
  {
    ...BASE_ANOMALY,
    id: 'rev-b-1',
    bug_id: 0,
    test_path: 'Master/BotA/BenchX/Test1/Sub',
    start_revision: 150,
    end_revision: 250,
  },

  // --- REVISION GROUP C: Disjoint / Singles (800-900) ---
  {
    ...BASE_ANOMALY,
    id: 'single-1',
    bug_id: 0,
    test_path: 'Master/BotA/BenchZ/Test9/Sub',
    start_revision: 800,
    end_revision: 900,
  },
  {
    ...BASE_ANOMALY,
    id: 'single-2',
    bug_id: 0,
    test_path: 'Master/BotA/BenchZ/Test9/Sub',
    start_revision: 950,
    end_revision: 1000, // Disjoint from single-1
  },
];

export const anomaly_table_for_tooltip: Anomaly[] = [
  {
    ...BASE_ANOMALY,
    id: 'multiple-bugs',
    bug_id: 12345,
    test_path: 'Master/BotA/BenchX/Test1/Sub',
    start_revision: 100,
    end_revision: 200,
    bugs: [
      {
        bug_id: '12345',
        bug_type: BugType('manual'),
      },
      {
        bug_id: '67890',
        bug_type: BugType('auto-triage'),
      },
      {
        bug_id: '11121',
        bug_type: BugType('auto-bisect'),
      },
      {
        bug_id: '11122',
        bug_type: BugType('auto-bisect'),
      },
      {
        bug_id: '11123',
        bug_type: BugType('auto-bisect'),
      },
      {
        bug_id: '11124',
        bug_type: BugType('auto-bisect'),
      },
    ],
  },
  {
    ...BASE_ANOMALY,
    id: 'single-bug',
    bug_id: 54321,
    test_path: 'Master/BotB/BenchY/Test2/Sub',
    start_revision: 800,
    end_revision: 900,
    bugs: [
      {
        bug_id: '54321',
        bug_type: BugType('auto-triage'),
      },
    ],
  },
  {
    ...BASE_ANOMALY,
    id: 'no-bugs',
    bug_id: 0,
    test_path: 'Master/BotC/BenchZ/Test3/Sub',
    start_revision: 1000,
    end_revision: 1100,
  },
];

export const GROUP_REPORT_RESPONSE_MULTIPLE_BUGS: GetGroupReportResponse = {
  sid: 'multiple-bugs-sid',
  anomaly_list: [anomaly_table_for_tooltip[0]],
  timerange_map: {},
  selected_keys: null,
  error: '',
  is_commit_number_based: true,
};

export const GROUP_REPORT_RESPONSE_SINGLE_BUG: GetGroupReportResponse = {
  sid: 'single-bug-sid',
  anomaly_list: [anomaly_table_for_tooltip[1]],
  timerange_map: {},
  selected_keys: null,
  error: '',
  is_commit_number_based: true,
};

export const GROUP_REPORT_RESPONSE_EMPTY_BUGS: GetGroupReportResponse = {
  sid: 'empty-bugs-sid',
  anomaly_list: [anomaly_table_for_tooltip[2]],
  timerange_map: {},
  selected_keys: null,
  error: '',
  is_commit_number_based: true,
};
