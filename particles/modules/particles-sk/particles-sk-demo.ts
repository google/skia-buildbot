import './index';
import './particles-sk-demo.css';

// eslint-disable-next-line import/no-extraneous-dependencies
import fetchMock from 'fetch-mock';
import { spiral } from './test_data';

const state = {
  filename: 'spiral.json',
  json: spiral,
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
fetchMock.spy('glob:*.wasm');
