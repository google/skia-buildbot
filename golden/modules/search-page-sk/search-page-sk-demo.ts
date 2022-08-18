import './index';
import '../gold-scaffold-sk';
import { $$ } from 'common-sk/modules/dom';
import { deepCopy } from 'common-sk/modules/object';
import { toParamSet } from 'common-sk/modules/query';
import fetchMock, { MockResponseObject } from 'fetch-mock';
import { testOnlySetSettings } from '../settings';
import { SearchPageSk } from './search-page-sk';
import {
  searchResponse, statusResponse, paramSetResponse, groupingsResponse, fakeNow, changeListSummaryResponse,
} from './demo_data';
import {
  DigestStatus, SearchResult, TriageDelta, TriageRequest, TriageRequestV3, TriageResponse,
} from '../rpc_types';
import { GoldScaffoldSk } from '../gold-scaffold-sk/gold-scaffold-sk';
import { delay } from '../demo_util';

testOnlySetSettings({
  title: 'Skia Infra',
  defaultCorpus: 'infra',
  baseRepoURL: 'https://skia.googlesource.com/buildbot.git',
});
Date.now = () => fakeNow;

fetchMock.get('/json/v2/trstatus', statusResponse);
fetchMock.get('/json/v2/paramset', paramSetResponse!);
fetchMock.get('/json/v1/groupings', groupingsResponse);
fetchMock.get('/json/v2/changelist/gerrit/123456', changeListSummaryResponse);

// We simulate the search endpoint, but only take into account the negative/positive/untriaged
// search fields and limit/offset to keep things simple. This is enough to demo the single/bulk
// triage UI components and pagination behavior.
fetchMock.get('glob:/json/v2/search*', (url: string) => {
  const filteredSearchResponse = deepCopy(searchResponse);
  const queryParams = toParamSet(url.substring(url.indexOf('?') + 1));

  // Filter only by untriaged/positive/negative.
  filteredSearchResponse.digests = filteredSearchResponse.digests!.filter(
    (digest) => (digest!.status === 'untriaged' && queryParams.unt[0] === 'true')
      || (digest!.status === 'positive' && queryParams.pos[0] === 'true')
      || (digest!.status === 'negative' && queryParams.neg[0] === 'true'),
  );
  filteredSearchResponse.size = filteredSearchResponse.digests.length;

  // Apply limit and offset.
  const limit = parseInt(queryParams.limit[0]);
  const offset = parseInt(queryParams.offset[0]);
  filteredSearchResponse.digests = filteredSearchResponse.digests.slice(offset, offset + limit);
  filteredSearchResponse.offset = offset;

  return delay(filteredSearchResponse, 1000);
});

// The simulated triage endpoint will make changes to the results returned by the search endpoint
// so as to demo the single/bulk triage UI components.
fetchMock.post('/json/v3/triage', (_: any, req: any) => {
  const triageRequest = JSON.parse(req.body as string) as TriageRequestV3;

  triageRequest.deltas.forEach((delta: TriageDelta) => {
    searchResponse.digests!.forEach((searchResult: SearchResult | null) => {
      // Update the search result if it matches the current digest.
      if (searchResult?.digest === delta.digest && searchResult.test === delta.grouping.name) {
        searchResult.status = delta.label_after;
      }

      // Update the label of the current digest if it appears in this search result's traces.
      searchResult?.traces.digests?.forEach((traceDigest: DigestStatus) => {
        if (traceDigest.digest === delta.digest) {
          traceDigest.status = delta.label_after;
        }
      });

      // Update negative reference image's label if it matches the current digest.
      const neg = searchResult?.refDiffs?.neg;
      if (neg?.digest === delta.digest) {
        neg.status = delta.label_after;
      }

      // Update positive reference image's label if it matches the current digest.
      const pos = searchResult?.refDiffs?.pos;
      if (pos?.digest === delta.digest) {
        pos.status = delta.label_after;
      }
    });
  });

  const response: TriageResponse = { status: 'ok' };
  const mro: MockResponseObject = { status: 200, body: response };
  return mro;
});

const newScaf = new GoldScaffoldSk();
newScaf.testingOffline = true;
const body = $$('body');
body?.insertBefore(newScaf, body.childNodes[0]); // Make it the first element in body.
const page = new SearchPageSk();
newScaf.appendChild(page);
