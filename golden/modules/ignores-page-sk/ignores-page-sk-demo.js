import './index';
import '../gold-scaffold-sk';

import { $$ } from 'common-sk/modules/dom';
import { delay } from '../demo_util';
import { ignoreRules_10, fakeNow } from './test_data';
import { manyParams } from '../shared_demo_data';
import { testOnlySetSettings } from '../settings';

const fetchMock = require('fetch-mock');

Date.now = () => fakeNow;
testOnlySetSettings({
  title: 'Skia Public',
});
$$('gold-scaffold-sk')._render(); // pick up title from settings.

fetchMock.get('/json/paramset', delay(manyParams, 100));
fetchMock.get('/json/ignores?counts=1', delay(ignoreRules_10, 300));
fetchMock.post('glob:/json/ignores/del/*', delay({}, 600));
fetchMock.post('glob:/json/ignores/add/', delay({}, 600));
fetchMock.post('glob:/json/ignores/save/*', delay({}, 600));
fetchMock.catch(404);
