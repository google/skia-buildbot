import './index';
import 'elements-sk/error-toast-sk';
import fetchMock from 'fetch-mock';

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

fetchMock.get('/_/initpage/?tz=America/New_York', () => ({
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

fetchMock.post('/_/cidRange/', () => [
  {
    offset: 43389,
    author: 'Avinash Parchuri (aparchur@google.com)',
    message:
      '3a543aa - 23h 34m - Reland "[skottie] Add onTextProperty support into ',
    url:
      'https://skia.googlesource.com/skia/+show/3a543aafd4e68af182ef88572086c094cd63f0b2',
    hash: '3a543aafd4e68af182ef88572086c094cd63f0b2',
    ts: 1565099441,
  },
  {
    offset: 43390,
    author: 'Robert Phillips (robertphillips@google.com)',
    message:
      'bdb0919 - 21h 15m - Use GrComputeTightCombinedBufferSize in GrMtlGpu::',
    url:
      'https://skia.googlesource.com/skia/+show/bdb0919dcc6a700b41492c53ecf06b40983d13d7',
    hash: 'bdb0919dcc6a700b41492c53ecf06b40983d13d7',
    ts: 1565107798,
  },
  {
    offset: 43391,
    author: 'Hal Canary (halcanary@google.com)',
    message:
      'e45bf6a - 20h 33m - experimental/editor: interface no longer uses stri',
    url:
      'https://skia.googlesource.com/skia/+show/e45bf6a603b7990f418eaf19ef0e2a2e59a9f449',
    hash: 'e45bf6a603b7990f418eaf19ef0e2a2e59a9f449',
    ts: 1565110328,
  },
]);

fetchMock.get('https://skia.org/loginstatus/', () => ({
  Email: 'jcgregorio@google.com',
  ID: '110642259984599645813',
  LoginURL: 'https://accounts.google.com/...',
  IsAGoogler: true,
  IsAdmin: true,
  IsEditor: false,
  IsViewer: true,
}));

customElements.whenDefined('cluster-page-sk').then(() => {
  // Insert the element later, which should given enough time for fetchMock to be in place.
  document
    .querySelector('h1')!
    .insertAdjacentElement(
      'afterend',
      document.createElement('cluster-page-sk'),
    );
});

window.sk = {
  perf: {
    commit_range_url: '',
    key_order: ['config'],
    demo: true,
    radius: 7,
    num_shift: 10,
    interesting: 25,
    step_up_only: false,
  },
};
