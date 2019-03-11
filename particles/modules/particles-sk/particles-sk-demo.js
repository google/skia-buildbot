import './index.js'
import './particles-sk-demo.css'

import { spiral } from './test_data.js'

const fetchMock = require('fetch-mock');

let state = {
  filename: 'spiral.json',
  json: spiral,
}
fetchMock.get('glob:/_/j/*', {
  status: 200,
  body: JSON.stringify(state),
  headers: {'Content-Type':'application/json'},
});

fetchMock.post('glob:/_/upload', {
  status: 200,
  body: JSON.stringify({
    hash: 'MOCK_UPLOADED',
  }),
  headers: {'Content-Type':'application/json'},
});

// Pass-through CanvasKit.
fetchMock.get('glob:*.wasm', fetchMock.realFetch.bind(window));