import './index';
import '../gold-scaffold-sk';

import { $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import { testOnlySetSettings } from '../settings';
import { delay } from '../demo_util';
import { clusterDiffJSON } from '../cluster-digests-sk/test_data';

testOnlySetSettings({
  title: 'Skia Demo',
  defaultCorpus: 'infra',
});
console.log('default set')
$$('gold-scaffold-sk')._render(); // pick up title from settings.

const fakeRpcDelayMillis = 300;

fetchMock.get('glob:/json/clusterdiff*', delay(clusterDiffJSON, fakeRpcDelayMillis));
fetchMock.get('/json/paramset', delay(clusterDiffJSON.paramsetsUnion, fakeRpcDelayMillis));
