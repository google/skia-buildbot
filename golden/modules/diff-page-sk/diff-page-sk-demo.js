import './index';
import '../gold-scaffold-sk';

import { typicalDetails, fakeNow } from '../digest-details-sk/test_data';
import { delay, isPuppeteerTest } from '../demo_util';
import { setImageEndpointsForDemos } from '../common';
import { $$ } from 'common-sk/modules/dom';
import { testOnlySetSettings } from '../settings';

const fetchMock = require('fetch-mock');

testOnlySetSettings({
  title: 'Skia Public',
});
$$('gold-scaffold-sk')._render(); // pick up title from settings.

setImageEndpointsForDemos();

// Load the demo page with some params to load if there aren't any already.
if (window.location.search.length < 4) {
  const query = '?left=6246b773851984c726cb2e1cb13510c2&right=99c58c7002073346ff55f446d47d6311&'+
    'test=My%20test%20has%20spaces&changelist_id=12353&crs=gerrit';
  history.pushState(null, '', window.location.origin + window.location.pathname + query);
}

const leftDetails = JSON.parse(JSON.stringify(typicalDetails));
const rightDetails = typicalDetails.refDiffs.pos;

// the server doesn't fill these out for the diff endpoint.
leftDetails.traces = null;
leftDetails.refDiffs = null;

Date.now = () => fakeNow;

const rpcDelay = isPuppeteerTest() ? 5 : 300;

fetchMock.get('glob:/json/diff*', delay(() => {
  if ($$('#simulate-rpc-error').checked) {
    return 500;
  }
  return JSON.stringify({
    left: leftDetails,
    right: rightDetails,
  });
}, rpcDelay));
fetchMock.catch(404);

// make the page reload when checkboxes change.
document.addEventListener('change', () => {
  $$('diff-page-sk')._fetch();
});

$$('#remove_btn').addEventListener('click', () => {
  const ele = $$('diff-page-sk');
  ele._changeListID = '';
  ele._render();
});
