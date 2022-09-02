import './index';
import '../gold-scaffold-sk';

import { $$ } from 'common-sk/modules/dom';
import fetchMock from 'fetch-mock';
import { toObject } from 'common-sk/modules/query';
import { HintableObject } from 'common-sk/modules/hintable';
import { fakeNow, makeTypicalSearchResult } from '../digest-details-sk/test_data';
import { delay } from '../demo_util';
import { testOnlySetSettings } from '../settings';
import { exampleStatusData } from '../last-commit-sk/demo_data';
import { GoldScaffoldSk } from '../gold-scaffold-sk/gold-scaffold-sk';
import { DiffPageSk } from './diff-page-sk';
import { setQueryString } from '../../../infra-sk/modules/test_util';
import { DigestComparison, LeftDiffInfo } from '../rpc_types';
import { groupingsResponse } from '../search-page-sk/demo_data';

testOnlySetSettings({
  title: 'Skia Public',
  baseRepoURL: 'https://github.com/flutter/flutter',
});

// Load the demo page with some params to load if there aren't any already.
if (window.location.search.length === 0) {
  setQueryString(
    '?left=6246b773851984c726cb2e1cb13510c2&right=99c58c7002073346ff55f446d47d6311&'
      + 'grouping=name%3DThis%2520is%2520a%2520test%2520with%2520spaces%26source_type%3Dinfra&'
      + 'changelist_id=12353&crs=gerrit',
  );
}

Date.now = () => fakeNow;

interface UrlParams {
  test: string;
  left: string;
  right: string;
  changelist_id?: string;
  crs?: string;
}

fetchMock.getOnce('/json/v1/groupings', groupingsResponse);
fetchMock.get('glob:/json/v2/diff*', (url) => {
  if ($$<HTMLInputElement>('#simulate-rpc-error')!.checked) {
    return delay(500);
  }

  // Make a response based on the URL parameters. This is needed by the Puppeteer test.
  const hint: UrlParams = {
    test: '', left: '', right: '', changelist_id: '', crs: '',
  };
  const urlParams = toObject(url.split('?')[1], hint as unknown as HintableObject) as unknown as UrlParams;
  const searchResult = makeTypicalSearchResult(urlParams.test, urlParams.left, urlParams.right);
  const leftInfo: LeftDiffInfo = {
    test: searchResult.test,
    digest: searchResult.digest,
    status: searchResult.status,
    triage_history: searchResult.triage_history,
    paramset: searchResult.paramset,
  };
  const response: DigestComparison = {
    left: leftInfo,
    right: searchResult.refDiffs!.pos!,
  };
  return delay(response);
});

fetchMock.get('/json/v2/trstatus', exampleStatusData);

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
