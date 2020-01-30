import './index.js'
import { $ } from 'common-sk/modules/dom'

let container = document.createElement('div');
document.body.appendChild(container);

afterEach(function() {
  container.innerHTML = '';
});

const paramset = {
  'arch': [
    'WASM',
    'arm',
    'arm64',
    'asmjs',
    'wasm',
    'x86',
    'x86_64'
  ],
  'bench_type': [
    'deserial',
    'micro',
    'playback',
    'recording',
    'skandroidcodec',
    'skcodec',
    'tracing'
  ],
  'compiler': [
    'Clang',
    'EMCC',
    'GCC',
  ],
  'config': [
    '8888',
    'f16',
    'gl',
    'gles',
  ],
};

describe('query-sk', () => {
  it('obeys key_order', () => {
    return window.customElements.whenDefined('query-sk').then(() => {
      container.innerHTML = `<query-sk></query-sk>`;
      const q = container.firstElementChild;
      q.paramset = paramset;
      assert.deepEqual(['arch', 'bench_type', 'compiler', 'config'], $('select-sk div', q).map((ele) => ele.textContent));

      // Setting key_order will change the key order.
      q.key_order = ['config'];
      assert.deepEqual(['config', 'arch', 'bench_type', 'compiler'], $('select-sk div', q).map((ele) => ele.textContent));

      // Setting key_order to empty will go back to alphabetical order.
      q.key_order = [];
      assert.deepEqual(['arch', 'bench_type', 'compiler', 'config'], $('select-sk div', q).map((ele) => ele.textContent));
    });
  });

  it('obeys filter', () => {
    return window.customElements.whenDefined('query-sk').then(() => {
      container.innerHTML = `<query-sk></query-sk>`;
      const q = container.firstElementChild;
      q.paramset = paramset;
      assert.deepEqual(['arch', 'bench_type', 'compiler', 'config'], $('select-sk div', q).map((ele) => ele.textContent));

      // Setting the filter will change the keys displayed.
      const fast = q.querySelector('#fast');
      fast.value = 'cro'; // Only 'micro' in 'bench_type' should match.
      fast.dispatchEvent(new Event('input')); // Emulate user input.

      // Only key should be bench_type.
      assert.deepEqual(['bench_type'], $('select-sk div', q).map((ele) => ele.textContent));

      // Clearing the filter will restore all options.
      fast.value = '';
      fast.dispatchEvent(new Event('input')); // Emulate user input.

      assert.deepEqual(['arch', 'bench_type', 'compiler', 'config'], $('select-sk div', q).map((ele) => ele.textContent));
    });
  });
});
