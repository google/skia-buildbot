import './index';
import '../gold-scaffold-sk';

import { $$ } from 'common-sk/modules/dom';
import { typicalDetails, fakeNow, twoHundredCommits } from '../digest-details-sk/test_data';
import { delay, isPuppeteerTest } from '../demo_util';
import { setImageEndpointsForDemos } from '../common';
import { testOnlySetSettings } from '../settings';
import { exampleStatusData } from '../last-commit-sk/demo_data';
import fetchMock from 'fetch-mock';

testOnlySetSettings({
  title: 'Skia Public',
  baseRepoURL: 'https://skia.googlesource.com/skia.git',
});

setImageEndpointsForDemos();

// Load the demo page with some params to load if there aren't any already. 4 is an arbitrary
// small number to check against to determine "no query params"
if (window.location.search.length < 4) {
  const query = '?digest=6246b773851984c726cb2e1cb13510c2&test=My%20test%20has%20spaces&changelist_id=12353&crs=gerrit-internal';
  history.pushState(null, '', window.location.origin + window.location.pathname + query);
}

Date.now = () => fakeNow;

const rpcDelay = isPuppeteerTest() ? 5 : 300;

fetchMock.get('glob:/json/v1/details*', delay(() => {
  if ($$('#simulate-rpc-error').checked) {
    return 500;
  }
  if ($$('#simulate-not-found-in-index').checked) {
    return JSON.stringify({
      digest: {
        digest: '6246b773851984c726cb2e1cb13510c2',
        test: 'This test exists, but the digest does not',
        status: 'untriaged',
      },
      commits: twoHundredCommits,
      trace_comments: null,
    });
  }
  return JSON.stringify({
    digest: typicalDetails,
    commits: twoHundredCommits,
    trace_comments: null,
  });
}, rpcDelay));
fetchMock.get('/json/v1/trstatus', JSON.stringify(exampleStatusData));

// make the page reload when checkboxes change.
document.addEventListener('change', () => {
  $$('details-page-sk')._fetch();
});

$$('#remove_btn').addEventListener('click', () => {
  const ele = $$('details-page-sk');
  ele._changeListID = '';
  ele._render();
});

// By adding these elements after all the fetches are mocked out, they should load ok.
const newScaf = document.createElement('gold-scaffold-sk');
newScaf.setAttribute('testing_offline', 'true');
const body = $$('body');
body.insertBefore(newScaf, body.childNodes[0]); // Make it the first element in body.
const page = document.createElement('details-page-sk');
newScaf.appendChild(page);
