import './index';
import '../../../elements-sk/modules/error-toast-sk';
import fetchMock from 'fetch-mock';
import { Alert, SerializesToString } from '../json';

window.perf = window.perf || {};
window.perf.instance_name = 'demo';
window.perf.header_image_url = '';
window.perf.key_order = [];
window.perf.display_group_by = true;
window.perf.notifications = 'markdown_issuetracker';

fetchMock.get('/_/login/status', {
  email: 'someone@example.org',
  roles: ['editor'],
});

fetchMock.post(
  '/_/count/',
  // Wait 1s before returning the content so we can see the spinner in action.
  async () =>
    new Promise((res) => setTimeout(() => res({ count: Math.floor(Math.random() * 2000) }), 1000))
);

fetchMock.post('/_/alert/update', 200);

// eslint-disable-next-line no-use-before-define
fetchMock.get('/_/alert/list/false', (): Alert[] => [
  {
    id_as_string: '5646874153320448',
    display_name: 'Image',
    query: 'source_type=image\u0026sub_result=min_ms',
    issue_tracker_component: SerializesToString('720614'),
    alert: '',
    step: 'cohen',
    interesting: 50,
    bug_uri_template: '',
    algo: 'stepfit',
    state: 'ACTIVE',
    owner: 'jcgregorio@google.com',
    step_up_only: false,
    direction: 'BOTH',
    radius: 7,
    k: 0,
    group_by: '',
    sparse: false,
    minimum_num: 0,
    category: ' ',
    action: 'noaction',
  },
]);

// eslint-disable-next-line no-use-before-define
fetchMock.get('/_/alert/list/true', (): Alert[] => [
  {
    id_as_string: '5646874153320448',
    display_name: 'Image',
    query: 'source_type=image\u0026sub_result=min_ms',
    issue_tracker_component: SerializesToString('720614'),
    alert: '',
    interesting: 50,
    bug_uri_template: '',
    algo: 'stepfit',
    state: 'ACTIVE',
    step: 'cohen',
    owner: 'jcgregorio@google.com',
    step_up_only: false,
    direction: 'BOTH',
    radius: 7,
    k: 0,
    group_by: '',
    sparse: false,
    minimum_num: 0,
    category: ' ',
    action: 'noaction',
  },
  {
    id_as_string: '2',
    display_name: 'Foo',
    query: 'source_type=image\u0026sub_result=min_ms',
    issue_tracker_component: SerializesToString('720614'),
    alert: '',
    interesting: 50,
    bug_uri_template: '',
    algo: 'stepfit',
    state: 'DELETED',
    owner: 'jcgregorio@google.com',
    step_up_only: false,
    step: 'mannwhitneyu',
    direction: 'BOTH',
    radius: 7,
    k: 0,
    group_by: '',
    sparse: false,
    minimum_num: 0,
    category: 'Stuff',
    action: 'noaction',
  },
]);

fetchMock.get('/_/initpage/', () => ({
  dataframe: {
    traceset: null,
    header: null,
    paramset: {
      arch: ['WASM', 'arm', 'arm64', 'asmjs', 'wasm', 'x86', 'x86_64'],
      bench_type: [
        'BRD',
        'deserial',
        'micro',
        'playback',
        'recording',
        'skandroidcodec',
        'skcodec',
        'tracing',
      ],
      browser: ['Chrome'],
      clip: ['0_0_1000_1000'],
      compiled_language: ['asmjs', 'wasm'],
      compiler: ['Clang', 'EMCC', 'GCC', 'MSVC', 'emsdk', 'none'],
      config: [
        '8888',
        'angle_d3d11_es2',
        'angle_d3d11_es2_msaa8',
        'angle_gl_es2',
        'angle_gl_es2_msaa8',
        'commandbuffer',
        'default',
        'enarrow',
        'esrgb',
        'f16',
        'gl',
        'gles',
        'glesmsaa4',
        'glessrgb',
        'glmsaa4',
        'glmsaa8',
        'glsrgb',
        'meta',
        'mtl',
      ],
      configuration: ['Debug', 'Presubmit', 'Release', 'devrel', 'eng', 'sdk'],
      cpu_or_gpu: ['CPU', 'GPU'],
    },
    skip: 0,
  },
  ticks: [],
  skps: [],
  msg: '',
}));

fetchMock.get(
  '/_/alert/new',
  // eslint-disable-next-line no-use-before-define
  (): Alert => ({
    id_as_string: '-1',
    display_name: 'Name',
    query: '',
    issue_tracker_component: SerializesToString(''),
    alert: '',
    step: 'cohen',
    interesting: 0,
    bug_uri_template: '',
    algo: 'kmeans',
    state: 'DELETED',
    owner: '',
    step_up_only: false,
    direction: 'BOTH',
    radius: 10,
    k: 50,
    group_by: '',
    sparse: false,
    minimum_num: 0,
    category: 'Experimental',
    action: 'noaction',
  })
);

// Insert the element later, which should given enough time for fetchMock to be in place.
customElements.whenDefined('alerts-page-sk').then(() => {
  document.querySelectorAll('h1').forEach((header) => {
    header.insertAdjacentElement('afterend', document.createElement('alerts-page-sk'));
  });
});
