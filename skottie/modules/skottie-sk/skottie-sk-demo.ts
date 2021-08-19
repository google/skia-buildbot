import './index';
import './skottie-sk-demo.css';

import fetchMock from 'fetch-mock';
import { $$ } from 'common-sk/modules/dom';
import { gear } from './test_gear';
import { SkottieSk } from './skottie-sk';

// TODO(kjlubick) for puppeteer tests, make this read in the hash and serve the appropriate JSON.
const state = {
  filename: 'moving_image.json',
  lottie: gear,
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
    lottie: gear,
  }),
  headers: { 'Content-Type': 'application/json' },
});

$$<SkottieSk>('skottie-sk')!.overrideAssetsPathForTesting(
  'https://storage.googleapis.com/skia-cdn/test_external_assets',
);
