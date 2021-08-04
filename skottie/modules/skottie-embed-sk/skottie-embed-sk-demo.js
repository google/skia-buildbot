import './index';

import fetchMock from 'fetch-mock';
import { gear } from './test_gear';

const state = {
  filename: 'gear.json',
  lottie: gear,
  width: 200,
  height: 200,
  fps: 30,
};
fetchMock.get('glob:/_/j/*', {
  status: 200,
  body: JSON.stringify(state),
  headers: { 'Content-Type': 'application/json' },
});
fetchMock.post('glob:/_/upload', {
  status: 200,
  body: JSON.stringify({
    hash: 'MOCK_UPLOADED',
  }),
  headers: { 'Content-Type': 'application/json' },
});

// Pass-through CanvasKit.
fetchMock.get('glob:*.wasm', fetchMock.realFetch.bind(window));
