// Functions to be used by test and demo files. These helps create dummy data and API mocks.
import fetchMock from 'fetch-mock';
import { Status } from '../../../infra-sk/modules/json';
import { QueryConfig, GetGroupReportResponse } from '../json';

const NEXT_PARAM_COUNT = 4;

const getCookieValue = (name: string) =>
  document.cookie.match('(^|;)\\s*' + name + '\\s*=\\s*([^;]+)')?.pop() || '';

export const paramSet = {
  arch: ['arm', 'arm64', 'x86_64'],
  os: ['Android', 'Debian10', 'Debian11', 'Mac10.13', 'Win2019', 'Ubuntu'],
};

export const anomalyTable = [
  {
    id: '123',
    test_path: ',arch=arm,os=Android,',
    bug_id: 0,
    start_revision: 67129,
    end_revision: 67130,
    is_improvement: false,
    recovered: false,
    state: 'untriaged',
    statistic: 'avg',
    units: 'ms',
    degrees_of_freedom: 1,
    median_before_anomaly: 60.830208,
    median_after_anomaly: 75.2,
    p_value: 0.01,
    segment_size_after: 10,
    segment_size_before: 10,
    std_dev_before_anomaly: 1,
    t_statistic: 5,
    subscription_name: 'test',
    bug_component: '',
    bug_labels: [],
    bug_cc_emails: [],
    bisect_ids: [],
  },
  {
    id: '456',
    test_path: ',arch=arm,os=Ubuntu,',
    bug_id: 0,
    start_revision: 67129,
    end_revision: 67130,
    is_improvement: false,
    recovered: false,
    state: 'untriaged',
    statistic: 'avg',
    units: 'ms',
    degrees_of_freedom: 1,
    median_before_anomaly: 13.464116,
    median_after_anomaly: 18.2,
    p_value: 0.01,
    segment_size_after: 10,
    segment_size_before: 10,
    std_dev_before_anomaly: 1,
    t_statistic: 5,
    subscription_name: 'test',
    bug_component: '',
    bug_labels: [],
    bug_cc_emails: [],
    bisect_ids: [],
  },
];

const GROUP_REPORT_RESPONSE: GetGroupReportResponse = {
  sid: '',
  anomaly_list: anomalyTable,
  timerange_map: {
    '123': { begin: 1687870256, end: 1687872763 },
    '456': { begin: 1687870256, end: 1687872763 },
  },
  selected_keys: ['123'], // Pre-select the first anomaly
  error: '',
  is_commit_number_based: true,
};

const STATUS_ID = 'd25fedcc-7e36-47e4-83d5-58ab76b2d3d1';

export const MOCK_TRACE_KEY_1 = ',arch=arm,os=Android,';
export const MOCK_TRACE_KEY_2 = ',arch=arm,os=Ubuntu,';
export const MOCK_TRACE_KEY_3 = ',arch=x86_64,os=Debian11,';

export const normalTracesResponse = {
  status: 'Finished',
  messages: [
    {
      key: 'Step',
      value: '1/1',
    },
  ],
  results: {
    dataframe: {
      traceset: {
        [MOCK_TRACE_KEY_1]: [
          61.2075, 60.687603, 61.30078, 61.660313, 60.830208, 75.2, 74.9, 75.3, 75.1, 75.4, 75.0,
          75.6, 75.2, 75.3, 75.1, 75.4, 75.0, 75.6, 75.2, 75.3, 75.1, 75.4, 75.0, 75.6, 75.2, 75.3,
          75.1, 75.4, 75.0, 75.6, 75.2, 75.3, 75.1, 75.4, 75.0, 75.6, 75.2, 75.3, 75.1, 75.4, 75.0,
          75.6, 75.2, 75.3, 75.1, 75.4, 75.0, 75.6, 75.2, 75.3,
        ],
        [MOCK_TRACE_KEY_2]: [
          13.51672, 13.434168, 13.47146, 13.494012, 13.464116, 18.2, 17.9, 18.3, 18.1, 18.4, 18.0,
          18.6, 18.2, 18.3, 18.1, 18.4, 18.0, 18.6, 18.2, 18.3, 18.1, 18.4, 18.0, 18.6, 18.2, 18.3,
          18.1, 18.4, 18.0, 18.6, 18.2, 18.3, 18.1, 18.4, 18.0, 18.6, 18.2, 18.3, 18.1, 18.4, 18.0,
          18.6, 18.2, 18.3, 18.1, 18.4, 18.0, 18.6, 18.2, 18.3, 18.1, 18.4,
        ],
        [MOCK_TRACE_KEY_3]: [
          10.51672, 10.434168, 10.47146, 10.494012, 10.464116, 15.2, 14.9, 15.3, 15.1, 15.4, 15.0,
          15.6, 15.2, 15.3, 15.1, 15.4, 15.0, 15.6, 15.2, 15.3, 15.1, 15.4, 15.0, 15.6, 15.2, 15.3,
          15.1, 15.4, 15.0, 15.6, 15.2, 15.3, 15.1, 15.4, 15.0, 15.6, 15.2, 15.3, 15.1, 15.4, 15.0,
          15.6, 15.2, 15.3, 15.1, 15.4, 15.0, 15.6, 15.2, 15.3, 15.1, 15.4,
        ],
      },
      header: [
        {
          offset: 67125,
          timestamp: 1687855198,
        },
        {
          offset: 67126,
          timestamp: 1687857789,
        },
        {
          offset: 67127,
          timestamp: 1687868015,
        },
        {
          offset: 67128,
          timestamp: 1687868368,
        },
        {
          offset: 67129,
          timestamp: 1687870256,
        },
        {
          offset: 67130,
          timestamp: 1687872763,
        },
        {
          offset: 67131,
          timestamp: 1687877748,
        },
        {
          offset: 67132,
          timestamp: 1687878083,
        },
        {
          offset: 67133,
          timestamp: 1687878588,
        },
        {
          offset: 67134,
          timestamp: 1687878658,
        },
        {
          offset: 67135,
          timestamp: 1687878976,
        },
        {
          offset: 67136,
          timestamp: 1687879230,
        },
        {
          offset: 67137,
          timestamp: 1687881375,
        },
        {
          offset: 67138,
          timestamp: 1687884748,
        },
        {
          offset: 67139,
          timestamp: 1687885047,
        },
        {
          offset: 67140,
          timestamp: 1687885507,
        },
        {
          offset: 67141,
          timestamp: 1687886132,
        },
        {
          offset: 67142,
          timestamp: 1687886787,
        },
        {
          offset: 67143,
          timestamp: 1687887013,
        },
        {
          offset: 67144,
          timestamp: 1687888513,
        },
        {
          offset: 67145,
          timestamp: 1687891891,
        },
        {
          offset: 67146,
          timestamp: 1687891925,
        },
        {
          offset: 67147,
          timestamp: 1687895229,
        },
        {
          offset: 67148,
          timestamp: 1687895693,
        },
        {
          offset: 67149,
          timestamp: 1687896092,
        },
        {
          offset: 67150,
          timestamp: 1687896114,
        },
        {
          offset: 67151,
          timestamp: 1687896459,
        },
        {
          offset: 67152,
          timestamp: 1687900291,
        },
        {
          offset: 67153,
          timestamp: 1687900389,
        },
        {
          offset: 67154,
          timestamp: 1687900992,
        },
        {
          offset: 67155,
          timestamp: 1687904682,
        },
        {
          offset: 67156,
          timestamp: 1687907669,
        },
        {
          offset: 67157,
          timestamp: 1687909158,
        },
        {
          offset: 67158,
          timestamp: 1687910749,
        },
        {
          offset: 67159,
          timestamp: 1687911636,
        },
        {
          offset: 67160,
          timestamp: 1687911698,
        },
        {
          offset: 67161,
          timestamp: 1687913983,
        },
        {
          offset: 67162,
          timestamp: 1687914369,
        },
        {
          offset: 67163,
          timestamp: 1687917173,
        },
        {
          offset: 67164,
          timestamp: 1687927827,
        },
        {
          offset: 67165,
          timestamp: 1687928532,
        },
        {
          offset: 67166,
          timestamp: 1687928754,
        },
        {
          offset: 67167,
          timestamp: 1687930648,
        },
        {
          offset: 67168,
          timestamp: 1687933565,
        },
        {
          offset: 67169,
          timestamp: 1687936673,
        },
        {
          offset: 67170,
          timestamp: 1687958245,
        },
        {
          offset: 67171,
          timestamp: 1687958371,
        },
        {
          offset: 67172,
          timestamp: 1687958912,
        },
        {
          offset: 67173,
          timestamp: 1687960354,
        },
        {
          offset: 67174,
          timestamp: 1687961972,
        },
      ],
      paramset: {
        arch: ['arm', 'arm64', 'x86_64'],
        os: ['Android', 'Debian10', 'Debian11', 'Mac10.13', 'Win2019', 'Ubuntu'],
      },
      skip: 0,
      traceMetadata: [],
      bands: [],
    },
    ticks: [],
    skps: [],
    msg: '',
    display_mode: 'display_plot',
    anomalymap: {
      [MOCK_TRACE_KEY_1]: {
        67130: anomalyTable[0],
      },
      [MOCK_TRACE_KEY_2]: {
        67130: anomalyTable[1],
      },
    },
  },
  url: `/_/status/${STATUS_ID}`,
};

export const defaultConfig: QueryConfig = {
  default_param_selections: null,
  default_url_values: null,
  include_params: ['arch', 'os'],
};

export function setUpExploreDemoEnv() {
  // June 28, 2023 at 14:19:33 UTC
  const RAW_TIMESTAMP_SECONDS = 1687961973;

  const MOCKED_NOW_MS = RAW_TIMESTAMP_SECONDS * 1000;

  Date.now = () => MOCKED_NOW_MS;

  const OriginalDate = Date;

  window.Date = class extends OriginalDate {
    constructor(...args: any[]) {
      if (args.length === 0) {
        super(MOCKED_NOW_MS);
      } else {
        // @ts-expect-error: type definition does not support spread arguments.
        super(...args);
      }
    }
  } as any;

  // The demo server will inject this cookie if there is a backend.
  if (getCookieValue('proxy_endpoint')) {
    return;
  }
  const status: Status = {
    email: 'user@google.com',
    roles: ['viewer', 'admin', 'editor', 'bisecter'],
  };
  fetchMock.post('/_/user_issues/', {
    UserIssues: [
      {
        UserId: 'user@google.com',
        TraceKey: MOCK_TRACE_KEY_1,
        CommitPosition: 67130,
        IssueId: 2345,
      },
    ],
  });

  fetchMock.get('/_/login/status', status);
  fetchMock.post('/_/anomalies/group_report', GROUP_REPORT_RESPONSE);

  fetchMock.get(/_\/initpage\/.*/, () => ({
    dataframe: {
      traceset: null,
      header: null,
      paramset: paramSet,
      skip: 0,
    },
    ticks: [],
    skps: [],
    msg: '',
  }));

  fetchMock.post('/_/count/', {
    count: 117, // Don't make the demo page non-deterministic.
    paramset: paramSet,
  });

  let currentQueries: string[] = [];
  let currentBegin = 1687855197;
  let currentEnd = 1687961973;

  fetchMock.post('/_/frame/start', (_url, opts) => {
    const body = JSON.parse(opts.body as string);
    currentQueries = body.queries || [];
    currentBegin = body.begin > 0 ? body.begin : currentBegin;
    currentEnd = body.end > 0 ? body.end : currentEnd;
    return {
      status: 'Running',
      messages: [],
      url: `/_/status/${STATUS_ID}`,
    };
  });

  fetchMock.get('/_/defaults/', defaultConfig);

  fetchMock.post('/_/nextParamList/', (_url, opts) => {
    const body = JSON.parse(opts.body as string);
    const q = body.q || '';
    const params = new URLSearchParams(q);

    // hierarchy order
    const order = ['arch', 'os'];

    let nextParam = '';
    for (let i = 0; i < order.length; i++) {
      if (!params.has(order[i])) {
        nextParam = order[i];
        break;
      }
    }

    // Construct response paramset with only the next param
    const responseParamSet: any = {};
    if (paramSet[nextParam as keyof typeof paramSet]) {
      responseParamSet[nextParam] = paramSet[nextParam as keyof typeof paramSet];
    }

    return {
      paramset: responseParamSet,
      count: NEXT_PARAM_COUNT,
    };
  });

  fetchMock.get(`/_/status/${STATUS_ID}`, () => {
    if (currentQueries.length === 0) {
      return {
        ...normalTracesResponse,
        results: {
          ...normalTracesResponse.results,
          dataframe: { ...normalTracesResponse.results.dataframe, traceset: {}, anomalymap: {} },
        },
      };
    }

    const df = normalTracesResponse.results.dataframe;

    let startIndex = df.header.findIndex((h) => h.timestamp >= currentBegin);
    if (startIndex === -1) {
      startIndex = df.header.length;
    }

    let endIndex = df.header.findIndex((h) => h.timestamp > currentEnd);
    if (endIndex === -1) {
      endIndex = df.header.length;
    }

    if (startIndex > endIndex) {
      startIndex = 0;
      endIndex = 0;
    }

    const slicedHeader = df.header.slice(startIndex, endIndex);

    let minOffset = -1;
    let maxOffset = -1;

    if (slicedHeader.length > 0) {
      minOffset = slicedHeader[0].offset;
      maxOffset = slicedHeader[slicedHeader.length - 1].offset;
    }

    const filteredTraceSet: any = {};
    const filteredAnomalyMap: any = {};

    Object.keys(df.traceset).forEach((traceKey) => {
      const matches = currentQueries.some((query) => {
        const params = new URLSearchParams(query);
        const queryMap = new Map<string, Set<string>>();
        for (const [key, value] of params) {
          if (!queryMap.has(key)) {
            queryMap.set(key, new Set());
          }
          queryMap.get(key)!.add(value);
        }

        for (const [key, values] of queryMap) {
          let keyMatch = false;
          for (const value of values) {
            if (traceKey.includes(`,${key}=${value},`)) {
              keyMatch = true;
              break;
            }
          }
          if (!keyMatch) return false;
        }
        return true;
      });

      if (matches) {
        const originalTrace = (df.traceset as any)[traceKey];
        filteredTraceSet[traceKey] = originalTrace.slice(startIndex, endIndex);

        const sourceAnomalies = (normalTracesResponse.results.anomalymap as any)?.[traceKey];

        if (sourceAnomalies) {
          const filteredTraceAnomalies: any = {};

          // Iterate over each anomaly (key is the revision offset)
          Object.keys(sourceAnomalies).forEach((revisionStr) => {
            const revision = Number(revisionStr);

            // Only include if the revision is within our visible header range
            if (revision >= minOffset && revision <= maxOffset) {
              filteredTraceAnomalies[revisionStr] = sourceAnomalies[revisionStr];
            }
          });

          // Only add to the map if we actually have anomalies left
          if (Object.keys(filteredTraceAnomalies).length > 0) {
            filteredAnomalyMap[traceKey] = filteredTraceAnomalies;
          }
        }
      }
    });

    return {
      ...normalTracesResponse,
      results: {
        ...normalTracesResponse.results,
        dataframe: {
          ...normalTracesResponse.results.dataframe,
          traceset: filteredTraceSet,
          header: slicedHeader,
        },
        anomalymap: filteredAnomalyMap,
      },
    };
  });

  fetchMock.post('/_/cid/', {
    commitSlice: [
      {
        offset: 67193,
        hash: '0d7087e5b99087f5945f04dbda7b7a7a4b12e344',
        ts: 1687990261,
        author: 'John Stiles (johnstiles@google.com)',
        message: 'Remove Win10 + ANGLE + IrisXe test and perf jobs.',
        url: 'https://skia.googlesource.com/skia/+show/0d7087e5b99087f5945f04dbda7b7a7a4b12e344',
        body: '',
      },
      {
        offset: 67194,
        hash: '2894e7194406ad8014d3e85b39379ca0e4607ead',
        ts: 1687991201,
        author: 'Arman Uguray (armansito@google.com)',
        message: 'Roll vello from ef2630ad to 12e764d5',
        url: 'https://skia.googlesource.com/skia/+show/2894e7194406ad8014d3e85b39379ca0e4607ead',
        body: '',
      },
    ],
    logEntry:
      'commit 0d7087e5b99087f5945f04dbda7b7a7a4b12e344\nAuthor John Stiles (johnstiles@google.com)\nDate 28 Jun 23 22:11 +0000\n\nRemove Win10 + ANGLE + IrisXe test and perf jobs.\n\nOnce skia:14417 is resolved, we should reinstate these jobs.\n\nBug: skia:14417\nChange-Id: Ib6b2a06cf7983c998d1d4e95a5e4973377b3bd48\nReviewed-on: https://skia-review.googlesource.com/c/skia/+/718157\nAuto-Submit: John Stiles \u003cjohnstiles@google.com\u003e\nCommit-Queue: Joe Gregorio \u003cjcgregorio@google.com\u003e\nCommit-Queue: John Stiles \u003cjohnstiles@google.com\u003e\nReviewed-by: Joe Gregorio \u003cjcgregorio@google.com\u003e\n',
  });

  fetchMock.post('/_/details/?results=false', {
    gitHash: 'e539c1a62d339f6509463a7e59d83141576e3722',
    key: {
      arch: 'arm',
      compiler: 'Clang',
      cpu_or_gpu: 'CPU',
      cpu_or_gpu_value: 'SnapdragonQM215',
      extra_config: 'Android',
      model: 'JioNext',
      os: 'Android',
    },
    swarming_bot_id: 'skia-rpi2-rack1-shelf1-026',
    swarming_task_id: '631c9c79d2c59211',
  });

  fetchMock.post('/_/shortcut/get', {
    GraphConfig: {
      queries: ['arch=arm64&bench_type=skandroidcodec'],
    },
  });

  fetchMock.post('/_/shortcut/update', {
    id: 'aaab78c9711cb79197d47f448ba51338',
  });

  fetchMock.post('/_/links/', {
    version: 1,
    links: {
      'Demo Link': 'https://example.com',
    },
  });

  fetchMock.post('/_/keys/', { id: 'test-key-id' });

  fetchMock.post('/_/fe_telemetry', {});
}
