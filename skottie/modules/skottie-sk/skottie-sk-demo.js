import './index.js'
import './skottie-sk-demo.css'

import { gear } from './test_gear.js'
const fetchMock = require('fetch-mock');

let state = {
  filename: 'gear.json',
  lottie: gear,
}
fetchMock.get('glob:/_/j/*', {
  status: 200,
  body: JSON.stringify(state),
  headers: {'Content-Type':'application/json'},
});

fetchMock.post('glob:/_/upload', {
  status: 200,
  body: JSON.stringify({
    hash: 'MOCK_UPLOADED'
  }),
  headers: {'Content-Type':'application/json'},
});

// Pass-through CanvasKit.
fetchMock.get('glob:*.wasm', fetchMock.realFetch.bind(window));
