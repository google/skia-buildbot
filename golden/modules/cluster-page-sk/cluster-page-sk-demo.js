import './index';
import '../gold-scaffold-sk';

import { $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import { testOnlySetSettings } from '../settings';
import { delay, isPuppeteerTest } from '../demo_util';
import { setImageEndpointsForDemos } from '../common';
import { clusterDiffJSON } from './test_data';
import { fakeNow, typicalDetails } from '../digest-details-sk/test_data';

testOnlySetSettings({
  title: 'Skia Demo',
  defaultCorpus: 'infra',
});
$$('gold-scaffold-sk')._render(); // pick up title from settings.

setImageEndpointsForDemos();

// Load the demo page with some params to load if there aren't any already. 4 is an arbitrary
// small number to check against to determine "no query params"
if (window.location.search.length < 4) {
  const query = '?grouping=dots-legend-sk_too-many-digests&issue=12353';
  history.pushState(null, '', window.location.origin + window.location.pathname + query);
}

Date.now = () => fakeNow;

const fakeRpcDelayMillis = isPuppeteerTest() ? 5 : 300;

fetchMock.get('glob:/json/clusterdiff*', delay(clusterDiffJSON, fakeRpcDelayMillis));
fetchMock.get('/json/paramset', delay(clusterDiffJSON.paramsetsUnion, fakeRpcDelayMillis));
fetchMock.get('glob:/json/details*', delay(typicalDetails, fakeRpcDelayMillis));

const leftDetails = JSON.parse(JSON.stringify(typicalDetails));
const rightDetails = typicalDetails.refDiffs.neg;

// The server doesn't fill these out for the diff endpoint.
leftDetails.traces = null;
leftDetails.refDiffs = null;

fetchMock.get('glob:/json/diff*', delay({
  left: leftDetails,
  right: rightDetails,
}, fakeRpcDelayMillis));
