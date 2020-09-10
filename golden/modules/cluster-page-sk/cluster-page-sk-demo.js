import './index';
import '../gold-scaffold-sk';

import { $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import { testOnlySetSettings } from '../settings';
import { delay, isPuppeteerTest } from '../demo_util';
import { setImageEndpointsForDemos } from '../common';
import { clusterDiffJSON } from './test_data';
import { fakeNow, twoHundredCommits, typicalDetails } from '../digest-details-sk/test_data';
import { exampleStatusData } from '../last-commit-sk/demo_data';

testOnlySetSettings({
  title: 'Skia Demo',
  defaultCorpus: 'infra',
  baseRepoURL: 'https://skia.googlesource.com/skia.git',
});

setImageEndpointsForDemos();

// Load the demo page with some params to load if there aren't any already. 4 is an arbitrary
// small number to check against to determine "no query params"
if (window.location.search.length < 4) {
  const query = '?grouping=dots-legend-sk_too-many-digests&changelist_id=12353&crs=gerrit';
  history.pushState(null, '', window.location.origin + window.location.pathname + query);
}

Date.now = () => fakeNow;

const fakeRpcDelayMillis = isPuppeteerTest() ? 5 : 300;

fetchMock.get('glob:/json/v1/clusterdiff*', delay(clusterDiffJSON, fakeRpcDelayMillis));
fetchMock.get('/json/v1/paramset', delay(clusterDiffJSON.paramsetsUnion, fakeRpcDelayMillis));
fetchMock.get('glob:/json/v1/details*', delay({
  digest: typicalDetails,
  commits: twoHundredCommits,
}, fakeRpcDelayMillis));
fetchMock.get('/json/v1/trstatus', JSON.stringify(exampleStatusData));

const leftDetails = JSON.parse(JSON.stringify(typicalDetails));
const rightDetails = typicalDetails.refDiffs.neg;

// The server doesn't fill these out for the diff endpoint.
leftDetails.traces = null;
leftDetails.refDiffs = null;

fetchMock.get('glob:/json/v1/diff*', delay({
  left: leftDetails,
  right: rightDetails,
}, fakeRpcDelayMillis));

// By adding these elements after all the fetches are mocked out, they should load ok.
const newScaf = document.createElement('gold-scaffold-sk');
newScaf.setAttribute('testing_offline', 'true');
const body = $$('body');
body.insertBefore(newScaf, body.childNodes[0]); // Make it the first element in body.
const page = document.createElement('cluster-page-sk');
newScaf.appendChild(page);
