import './index';
import '../gold-scaffold-sk';

import { fakeNow, makeTypicalSearchResult}  from '../digest-details-sk/test_data';
import { delay } from '../demo_util';
import { setImageEndpointsForDemos } from '../common';
import { $$ } from 'common-sk/modules/dom';
import { testOnlySetSettings } from '../settings';
import { exampleStatusData } from '../last-commit-sk/demo_data';
import fetchMock from 'fetch-mock';
import { GoldScaffoldSk } from '../gold-scaffold-sk/gold-scaffold-sk';
import { DiffPageSk } from './diff-page-sk';
import { setQueryString } from '../../../infra-sk/modules/test_util';
import { DigestComparison } from '../rpc_types';
import { toObject } from 'common-sk/modules/query';
import { HintableObject } from 'common-sk/modules/hintable';

testOnlySetSettings({
  title: 'Skia Public',
  baseRepoURL: 'https://github.com/flutter/flutter',
});

setImageEndpointsForDemos();

// Load the demo page with some params to load if there aren't any already.
if (window.location.search.length === 0) {
  setQueryString(
      '?left=6246b773851984c726cb2e1cb13510c2&right=99c58c7002073346ff55f446d47d6311&'
      + 'test=This%20is%20a%20test%20with%20spaces&changelist_id=12353&crs=gerrit');
}

Date.now = () => fakeNow;

interface UrlParams {
  test: string;
  left: string;
  right: string;
  changelist_id?: string;
  crs?: string;
}

fetchMock.get('glob:/json/v1/diff*', (url) => {
  if ($$<HTMLInputElement>('#simulate-rpc-error')!.checked) {
    return delay(500);
  }

  // Make a response based on the URL parameters. This is needed by the Puppeteer test.
  const hint: UrlParams = {test: '', left: '', right: '', changelist_id: '', crs: ''};
  const urlParams =
      toObject(url.split('?')[1], hint as unknown as HintableObject) as unknown as UrlParams;
  const searchResult = makeTypicalSearchResult(urlParams.test, urlParams.left, urlParams.right);
  const response: DigestComparison = {
    left: searchResult,
    right: searchResult.refDiffs.pos,
  }
  return delay(response);
})

fetchMock.get('/json/v1/trstatus', exampleStatusData);

// By adding these elements after all the fetches are mocked out, they should load ok.
const scaffold = new GoldScaffoldSk();
scaffold.testingOffline = true;
// Make it the first element in body.
document.body.insertBefore(scaffold, document.body.childNodes[0]);

let diffPageSk = new DiffPageSk();
scaffold.appendChild(diffPageSk);

document.querySelector('#simulate-rpc-error')!.addEventListener('change', () => {
  // Reload the page to trigger an RPC error.
  diffPageSk.parentNode!.removeChild(diffPageSk);
  diffPageSk = new DiffPageSk();
  scaffold.appendChild(diffPageSk);
});

