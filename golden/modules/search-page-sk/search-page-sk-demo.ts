import './index';
import '../gold-scaffold-sk';
import { $$ } from 'common-sk/modules/dom';
import { deepCopy } from 'common-sk/modules/object';
import { testOnlySetSettings } from '../settings';
import { SearchPageSk } from './search-page-sk';
import { searchResponse, statusResponse, paramSetResponse, fakeNow, changeListSummaryResponse } from './demo_data';
import fetchMock from 'fetch-mock';
import { setImageEndpointsForDemos } from '../common';
import { TriageRequest } from '../rpc_types';
import { GoldScaffoldSk } from '../gold-scaffold-sk/gold-scaffold-sk';

testOnlySetSettings({
  title: 'Skia Infra',
  defaultCorpus: 'infra',
  baseRepoURL: 'https://skia.googlesource.com/buildbot.git',
});
Date.now = () => fakeNow;

fetchMock.get('/json/v1/trstatus', statusResponse);
fetchMock.get('/json/v1/paramset', paramSetResponse!);
fetchMock.get('/json/v1/changelist/gerrit/123456', changeListSummaryResponse);

// We simulate the search endpoint, but only take into account the negative/positive/untriaged
// search fields to keep things simple. This is enough to demo the single/bulk triage UI components.
fetchMock.get('glob:/json/v1/search*', (url: string) => {
  const filteredSearchResponse = deepCopy(searchResponse);

  // Filter only by untriaged/positive/negative.
  filteredSearchResponse.digests = filteredSearchResponse.digests.filter(
    (digest) =>
      (digest!.status === 'untriaged' && url.includes('unt=true')) ||
      (digest!.status === 'positive' && url.includes('pos=true')) ||
      (digest!.status === 'negative' && url.includes('neg=true')));
  filteredSearchResponse.size = filteredSearchResponse.digests.length;

  return filteredSearchResponse
});

// The simulated triage endpoint will make changes to the results returned by the search endpoint
// so as to demo the single/bulk triage UI components.
fetchMock.post('/json/v1/triage', (_: any, req: any) => {
  const triageRequest: TriageRequest = JSON.parse(req.body as string);

  // Iterate over all digests in the triage request (same for single and bulk triage operations).
  Object.keys(triageRequest.testDigestStatus).forEach((testName) => {
    Object.keys(triageRequest.testDigestStatus[testName]).forEach((digest) => {
      const label = triageRequest.testDigestStatus[testName][digest];

      // Empty means "closest", which we ignore for simplicity. For more details, please see
      // https://github.com/google/skia-buildbot/blob/6dd58fac8d1eac7bbf4e737110605dcdf1b20a56/golden/modules/bulk-triage-sk/bulk-triage-sk.ts#L134
      //
      // TODO(lovisolo): Remove this guard once the notes in the above link have been addressed.
      if (label as string === '') return;

      // Iterate over all search results.
      searchResponse.digests.forEach((searchResult) => {
        // Update the search result if it matches the current digest.
        if (searchResult?.digest === digest && searchResult.test === testName) {
          searchResult.status = label;
        }

        // Update the label of the current digest if it appears in this search result's traces.
        searchResult?.traces.digests?.forEach((traceDigest) => {
          if (traceDigest.digest === digest) {
            traceDigest.status = label;
          }
        });

        // Update negative reference image's label if it matches the current digest.
        const neg = searchResult?.refDiffs?.neg;
        if (neg?.digest === digest) {
          neg.status = label;
        }

        // Update positive reference image's label if it matches the current digest.
        const pos =  searchResult?.refDiffs?.pos;
        if (pos?.digest === digest) {
          pos.status = label;
        }
      });
    })
  });

  return 200;
});

setImageEndpointsForDemos();

const newScaf = new GoldScaffoldSk();
newScaf.testingOffline = true;
const body = $$('body');
body?.insertBefore(newScaf, body.childNodes[0]); // Make it the first element in body.
const page = new SearchPageSk();
newScaf.appendChild(page);
