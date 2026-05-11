import './index';
import '../../../elements-sk/modules/error-toast-sk';
import { setUpExploreDemoEnv } from '../common/test-util';
import fetchMock from 'fetch-mock';

setUpExploreDemoEnv();
(window as any).fetchMock = fetchMock;
fetchMock.config.fallbackToNetwork = true;
(window as any).WORKER_URL = '/dist/explore-multi-v2-sk/filter.worker.bundle.js';

const numTraces = 10;
const numPoints = 100;
const stride = 10;

const params = [
  { id: 1, key: 'os', value: 'Android' },
  { id: 2, key: 'os', value: 'Ubuntu' },
  { id: 3, key: 'arch', value: 'arm' },
  { id: 4, key: 'arch', value: 'x86' },
  { id: 5, key: 'config', value: '8888' },
  { id: 6, key: 'config', value: 'gpu' },
];

const tracesBuffer = new Uint16Array(numTraces * stride);
for (let i = 0; i < numTraces; i++) {
  const offset = i * stride;
  tracesBuffer[offset] = (i % 2) + 1; // os
  tracesBuffer[offset + 1] = ((i >> 1) % 2) + 3; // arch
  tracesBuffer[offset + 2] = ((i >> 2) % 2) + 5; // config
}

const header = [];
for (let i = 0; i < numPoints; i++) {
  header.push({
    offset: 100 + i,
    timestamp: 1687855198 + i,
    hash: `abc${i}`,
    author: 'a',
    message: 'm',
    url: 'u',
  });
}

const traceset: Record<string, number[]> = {};
for (let i = 0; i < numTraces; i++) {
  const os = i % 2 === 0 ? 'Android' : 'Ubuntu';
  const arch = (i >> 1) % 2 === 0 ? 'arm' : 'x86';
  const config = (i >> 2) % 2 === 0 ? '8888' : 'gpu';
  const key = `,arch=${arch},config=${config},os=${os},project=Skia,`;
  traceset[key] = [];
  for (let j = 0; j < numPoints; j++) {
    traceset[key].push(10 + i + Math.sin(j / 10) * 2);
  }
}

const anomalymap: Record<string, Record<string, any>> = {};
const keys = Object.keys(traceset);

const firstKey = keys[0];
traceset[firstKey][50] = 30; // Regression
anomalymap[firstKey] = {
  '150': {
    id: '123',
    bug_id: 456,
    is_improvement: false,
    median_before_anomaly: 10.0,
    median_after_anomaly: 30.0,
  },
};

const secondKey = keys[1];
traceset[secondKey][50] = 5; // Improvement
anomalymap[secondKey] = {
  '150': {
    id: '124',
    bug_id: 0,
    is_improvement: true,
    median_before_anomaly: 15.0,
    median_after_anomaly: 5.0,
  },
};

const thirdKey = keys[2];
traceset[thirdKey][50] = 25; // Untriaged Regression
anomalymap[thirdKey] = {
  '150': {
    id: '125',
    bug_id: 0,
    is_improvement: false,
    median_before_anomaly: 15.0,
    median_after_anomaly: 25.0,
  },
};

fetchMock.get(
  'glob:*/_/wasm/meta.json*',
  { version: 'test-version', count: numTraces, stride: stride, commonParams: { project: 'Skia' } },
  { overwriteRoutes: true }
);
fetchMock.get('glob:*/_/wasm/params.json*', params, { overwriteRoutes: true });
fetchMock.get('glob:*/_/wasm/traces.bin*', new Response(tracesBuffer.buffer), {
  overwriteRoutes: true,
});

fetchMock.post(
  '/_/frame/start',
  {
    status: 'Finished',
    results: {
      dataframe: {
        traceset: traceset,
        header: header,
        paramset: {
          os: ['Android', 'Ubuntu'],
          arch: ['arm', 'x86'],
          config: ['8888', 'gpu'],
          project: ['Skia'],
        },
        skip: 0,
        traceMetadata: null,
      },
      anomalymap: anomalymap,
    },
  },
  { overwriteRoutes: true }
);

// Override defaults to enable test picker, which is required to attach the add-to-graph listener.
fetchMock.get(
  '/_/defaults/',
  {
    default_param_selections: null,
    default_url_values: { useTestPicker: 'true' },
    include_params: ['arch', 'os', 'test'],
  },
  { overwriteRoutes: true }
);

(window as any).perf = {
  dev_mode: false,
  instance_url: '',
  instance_name: 'chrome-perf-demo',
  header_image_url: '',
  commit_range_url: '',
  key_order: ['config'],
  demo: true,
  radius: 7,
  num_shift: 10,
  interesting: 25,
  step_up_only: false,
  display_group_by: true,
  hide_list_of_commits_on_explore: false,
  notifications: 'none',
  fetch_chrome_perf_anomalies: false,
  fetch_anomalies_from_sql: false,
  feedback_url: '',
  chat_url: '',
  help_url_override: '',
  trace_format: 'chrome',
  need_alert_action: false,
  bug_host_url: '',
  git_repo_url: '',
  keys_for_commit_range: [],
  keys_for_useful_links: [],
  skip_commit_detail_display: false,
  image_tag: 'fake-tag',
  remove_default_stat_value: false,
  enable_skia_bridge_aggregation: false,
  show_json_file_display: false,
  always_show_commit_info: false,
  show_triage_link: true,
  show_bisect_btn: true,
  app_version: 'test-version',
  enable_v2_ui: true,
  extra_links: null,
};

customElements
  .whenDefined('explore-multi-v2-sk')
  .then(() => {
    document
      .querySelector('h1')!
      .insertAdjacentElement('afterend', document.createElement('explore-multi-v2-sk'));
  })
  .catch(console.error);
