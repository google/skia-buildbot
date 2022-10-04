import './index';
import 'elements-sk/error-toast-sk';
import fetchMock from 'fetch-mock';
import { Alert } from '../json';

const paramSet = {
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
};

fetchMock.get('/_/initpage/', () => ({
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

const alert: Alert = {
  id_as_string: '-1',
  sparse: false,
  step_up_only: false,
  display_name: 'A name',
  direction: 'BOTH',
  query: '',
  alert: 'alerts@example.com',
  interesting: 25,
  step: 'cohen',
  bug_uri_template: 'http://example.com/{description}/{url}',
  algo: 'stepfit',
  owner: 'somebody@example.org',
  minimum_num: 1,
  category: '',
  state: 'ACTIVE',
  group_by: '',
  radius: 7,
  k: 50,
};

fetchMock.get('/_/alert/new', alert);

fetchMock.post('/_/count/', {
  count: Math.floor(Math.random() * 2000),
  paramset: paramSet,
});

customElements.whenDefined('cluster-lastn-page-sk').then(() => {
  // Insert the element later, which should given enough time for fetchMock to be in place.
  document
    .querySelector('h1')!
    .insertAdjacentElement(
      'afterend',
      document.createElement('cluster-lastn-page-sk'),
    );
});

window.perf = window.perf || {};
window.perf.key_order = [];
window.perf.demo = true;
