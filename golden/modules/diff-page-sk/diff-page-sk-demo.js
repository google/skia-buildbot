import './index';
import '../gold-scaffold-sk';

import { typicalDetails, fakeNow } from '../digest-details-sk/test_data';
import { delay, isPuppeteerTest } from '../demo_util';
import { setImageEndpointsForDemos } from '../common';
import { $$ } from 'common-sk/modules/dom';
import { testOnlySetSettings } from '../settings';
import { exampleStatusData } from '../last-commit-sk/demo_data';
import fetchMock from 'fetch-mock';

testOnlySetSettings({
  title: 'Skia Public',
  baseRepoURL: 'https://github.com/flutter/flutter',
});

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

fetchMock.get('glob:/json/v1/diff*', delay(() => {
  if ($$('#simulate-rpc-error').checked) {
    return 500;
  }
  return JSON.stringify({
    left: leftDetails,
    right: rightDetails,
  });
}, rpcDelay));
fetchMock.get('/json/v1/trstatus', JSON.stringify(exampleStatusData));

// make the page reload when checkboxes change.
document.addEventListener('change', () => {
  $$('diff-page-sk')._fetch();
});

$$('#remove_btn').addEventListener('click', () => {
  const ele = $$('diff-page-sk');
  ele._changeListID = '';
  ele._render();
});

// By adding these elements after all the fetches are mocked out, they should load ok.
const newScaf = document.createElement('gold-scaffold-sk');
newScaf.setAttribute('testing_offline', 'true');
const body = $$('body');
body.insertBefore(newScaf, body.childNodes[0]); // Make it the first element in body.
const page = document.createElement('diff-page-sk');
newScaf.appendChild(page);
