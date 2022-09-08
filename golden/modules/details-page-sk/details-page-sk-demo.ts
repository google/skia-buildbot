import './index';
import '../gold-scaffold-sk';

import { $$ } from 'common-sk/modules/dom';
import fetchMock from 'fetch-mock';
import { fakeNow, twoHundredCommits, makeTypicalSearchResult } from '../digest-details-sk/test_data';
import { delay } from '../demo_util';
import { testOnlySetSettings } from '../settings';
import { exampleStatusData } from '../last-commit-sk/demo_data';
import { GoldScaffoldSk } from '../gold-scaffold-sk/gold-scaffold-sk';
import { DetailsPageSk } from './details-page-sk';
import {
  DetailsRequest, DigestDetails, GroupingForTestRequest, GroupingForTestResponse,
} from '../rpc_types';
import { setQueryString } from '../../../infra-sk/modules/test_util';
import { groupingsResponse } from '../search-page-sk/demo_data';

testOnlySetSettings({
  title: 'Skia Public',
  baseRepoURL: 'https://skia.googlesource.com/skia.git',
});

// Load the demo page with some params to load if there aren't any already.
if (window.location.search.length === 0) {
  setQueryString(
    '?digest=6246b773851984c726cb2e1cb13510c2&'
    + 'grouping=name%3DThis%2520is%2520a%2520test%2520with%2520spaces%26source_type%3Dinfra&'
      + 'changelist_id=12353&crs=gerrit-internal',
  );
}

Date.now = () => fakeNow;

fetchMock.get('/json/v1/groupings', groupingsResponse);
fetchMock.post('/json/v1/groupingfortest', (url, opts) => {
  const request: GroupingForTestRequest = JSON.parse(opts.body!.toString());
  const response: GroupingForTestResponse = {
    grouping: {
      name: request.test_name,
      source_type: 'infra',
    },
  };
  return response;
});

fetchMock.post('/json/v2/details', (url, opts) => {
  if ($$<HTMLInputElement>('#simulate-rpc-error')!.checked) {
    return 500;
  }

  // Make a response based on the RPC request. This is needed by the Puppeteer test.
  const request: DetailsRequest = JSON.parse(opts.body!.toString());
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
  const closestDigest = request.digest === knownDigest1 ? knownDigest2 : knownDigest1;
  const response: DigestDetails = {
    digest: makeTypicalSearchResult(request.grouping.name, request.digest, closestDigest),
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
