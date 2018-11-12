
import './index.js'

import { gear } from './test_gear.js'
const fetchMock = require('fetch-mock');

let state = {
  filename: 'gear.json',
  lottie: gear,
  width: 200,
  height: 200,
  fps: 30,
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
