import fetchMock from 'fetch-mock';
import { $$ } from 'common-sk/modules/dom';
import { Commit } from '../json/all';
import 'elements-sk/error-toast-sk';

import { CommitDetailPickerSk } from '../commit-detail-picker-sk/commit-detail-picker-sk';
import { QuerySk } from '../../../infra-sk/modules/query-sk/query-sk';

window.perf = {
  commit_range_url: '',
  key_order: ['config'],
  demo: true,
  radius: 7,
  num_shift: 10,
  interesting: 25,
  step_up_only: false,
  display_group_by: true,
};

Date.now = () => Date.parse('2020-03-22T00:00:00.000Z');

fetchMock.post('/_/cidRange/', (): Commit[] => [
  {
    offset: 43389,
    author: 'Avinash Parchuri (aparchur@google.com)',
    message: 'Reland "[skottie] Add onTextProperty support into ',
    url:
      'https://skia.googlesource.com/skia/+show/3a543aafd4e68af182ef88572086c094cd63f0b2',
    hash: '3a543aafd4e68af182ef88572086c094cd63f0b2',
    ts: 1565099441,
  },
  {
    offset: 43390,
    author: 'Robert Phillips (robertphillips@google.com)',
    message: 'Use GrComputeTightCombinedBufferSize in GrMtlGpu::',
    url:
      'https://skia.googlesource.com/skia/+show/bdb0919dcc6a700b41492c53ecf06b40983d13d7',
    hash: 'bdb0919dcc6a700b41492c53ecf06b40983d13d7',
    ts: 1565107798,
  },
  {
    offset: 43391,
    author: 'Hal Canary (halcanary@google.com)',
    message: 'experimental/editor: interface no longer uses stri',
    url:
      'https://skia.googlesource.com/skia/+show/e45bf6a603b7990f418eaf19ef0e2a2e59a9f449',
    hash: 'e45bf6a603b7990f418eaf19ef0e2a2e59a9f449',
    ts: 1565220328,
  },
]);

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

fetchMock.post('/_/count/', {
  count: 117, // Don't make the demo page non-deterministic.
  paramset: paramSet,
});

fetchMock.get('path:/_/initpage/', () => ({
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

// eslint-disable-next-line import/first
import './index';

$$<QuerySk>('query-sk')!.current_query = 'config=8888';
$$<CommitDetailPickerSk>('commit-detail-picker-sk')!.selection = 43390;
fetchMock.flush(true).then(() => {
  $$<HTMLDivElement>('#load-complete')!.innerHTML = '<pre>Finished</pre>';
});
