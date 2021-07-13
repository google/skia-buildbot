import './index';
import '../gold-scaffold-sk';

import { $$ } from 'common-sk/modules/dom';
import fetchMock from 'fetch-mock';
import { toObject } from 'common-sk/modules/query';
import { HintableObject } from 'common-sk/modules/hintable';
import {
  fakeNow, twoHundredCommits, makeTypicalSearchResult,
} from '../digest-details-sk/test_data';
import { delay } from '../demo_util';
import { setImageEndpointsForDemos } from '../common';
import { testOnlySetSettings } from '../settings';
import { exampleStatusData } from '../last-commit-sk/demo_data';
import { GoldScaffoldSk } from '../gold-scaffold-sk/gold-scaffold-sk';
import { DetailsPageSk } from './details-page-sk';
import { DigestDetails } from '../rpc_types';
import { setQueryString } from '../../../infra-sk/modules/test_util';

testOnlySetSettings({
  title: 'Skia Public',
  baseRepoURL: 'https://skia.googlesource.com/skia.git',
});

setImageEndpointsForDemos();

// Load the demo page with some params to load if there aren't any already.
if (window.location.search.length === 0) {
  setQueryString(
    '?digest=6246b773851984c726cb2e1cb13510c2&test=This%20is%20a%20test%20with%20spaces&'
      + 'changelist_id=12353&crs=gerrit-internal',
  );
}

Date.now = () => fakeNow;

interface UrlParams {
  digest: string;
  test: string;
}

fetchMock.get('glob:/json/v1/details*', (url) => {
  if ($$<HTMLInputElement>('#simulate-rpc-error')!.checked) {
    return 500;
  }

  // Make a response based on the URL parameters. This is needed by the Puppeteer test.
  const hint: UrlParams = { digest: '', test: '' };
  const urlParams = toObject(url.split('?')[1], hint as unknown as HintableObject) as unknown as UrlParams;

  if ($$<HTMLInputElement>('#simulate-not-found-in-index')!.checked) {
    const response: DigestDetails = {
      digest: {
        digest: '6246b773851984c726cb2e1cb13510c2',
        test: 'This test exists, but the digest does not',
        status: 'untriaged',
        triage_history: [],
        paramset: {},
        traces: {
          traces: [],
          digests: [],
          total_digests: 0,
        },
        refDiffs: {},
        closestRef: '',
      },
      commits: twoHundredCommits,
    };
    return delay(response);
  }
  const knownDigest1 = '99c58c7002073346ff55f446d47d6311';
  const knownDigest2 = '6246b773851984c726cb2e1cb13510c2';
  const closestDigest = urlParams.digest === knownDigest1 ? knownDigest2 : knownDigest1;
  const response: DigestDetails = {
    digest: makeTypicalSearchResult(urlParams.test, urlParams.digest, closestDigest),
    commits: twoHundredCommits,
  };
  return delay(response);
});

fetchMock.get('/json/v2/trstatus', JSON.stringify(exampleStatusData));

// By adding these elements after all the fetches are mocked out, they should load ok.
const scaffold = new GoldScaffoldSk();
scaffold.testingOffline = true;
// Make it the first element in body.
document.body.insertBefore(scaffold, document.body.childNodes[0]);

let detailsPageSk = new DetailsPageSk();
scaffold.appendChild(detailsPageSk);

document.querySelectorAll('#simulate-rpc-error, #simulate-not-found-in-index')
  .forEach((el) => el.addEventListener('change', (e) => {
    e.stopPropagation();
      // Reload the page to trigger a new RPC.
      detailsPageSk.parentNode!.removeChild(detailsPageSk);
      detailsPageSk = new DetailsPageSk();
      scaffold.appendChild(detailsPageSk);
  }));
