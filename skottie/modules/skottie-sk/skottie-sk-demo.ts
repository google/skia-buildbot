import './index';

import fetchMock from 'fetch-mock';
import { $$ } from '../../../infra-sk/modules/dom';
import { gear, withText } from './test_gear';
import { SkottieSk } from './skottie-sk';

let lottieToServe = gear;
const params = new URLSearchParams(window.location.search);
if (params.get('test') === 'withText') {
  lottieToServe = withText;
}
const state = {
  filename: 'moving_image.json',
  lottie: lottieToServe,
};
const resources = {
  files: [],
};

fetchMock.config.fallbackToNetwork = true;
fetchMock.get('glob:/_/j/*', {
  status: 200,
  body: JSON.stringify(state),
  headers: { 'Content-Type': 'application/json' },
});

fetchMock.get('glob:/_/r/*', {
  status: 200,
  body: JSON.stringify(resources),
  headers: { 'Content-Type': 'application/json' },
});

fetchMock.post('glob:/_/upload', {
  status: 200,
  body: JSON.stringify({
    hash: 'MOCK_UPLOADED',
    lottie: lottieToServe,
  }),
  headers: { 'Content-Type': 'application/json' },
});

$$<SkottieSk>('skottie-sk')!.overrideAssetsPathForTesting(
  'https://storage.googleapis.com/skia-cdn/test_external_assets'
);
