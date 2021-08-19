import './index';

import fetchMock from 'fetch-mock';
import { gear } from '../skottie-sk/test_gear';

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
