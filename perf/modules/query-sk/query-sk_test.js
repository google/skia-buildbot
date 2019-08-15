import './index.js'

let container = document.createElement('div');
document.body.appendChild(container);

afterEach(function() {
  container.innerHTML = "";
});

const paramset = {
  "paramset": {
    "arch": [
      "WASM",
      "arm",
      "arm64",
      "asmjs",
      "wasm",
      "x86",
      "x86_64"
    ],
    "bench_type": [
      "deserial",
      "micro",
      "playback",
      "recording",
      "skandroidcodec",
      "skcodec",
      "tracing"
    ],
    "compiler": [
      "Clang",
      "EMCC",
      "GCC",
    ],
    "config": [
      "8888",
      "f16",
      "gl",
      "gles",
    ],
  }
};

describe('query-sk', () => {
  it('obeys key_order', () => {
    return window.customElements.whenDefined('query-sk').then(() => {
      container.innerHTML = `<query-sk></query-sk>`;
      const q = container.firstElementChild;
      q.paramset = paramset;
      q.key_order = ["config"];
      const firstKey = q.querySelector('select-sk').firstElementChild;
      assert.equal('config', firstKey.textContent, 'Element is changed.');
    });
  });
});
