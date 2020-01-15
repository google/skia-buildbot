import './index.js'
import '../gold-scaffold-sk'

import { delay } from '../demo_util'
import { ignoreRules_10, fakeNow } from './test_data'

const fetchMock = require('fetch-mock');

Date.now = () => fakeNow;

fetchMock.get('/json/ignores?counts=1', delay(ignoreRules_10, 300));
fetchMock.post('glob:/json/ignores/del/*', delay({}, 600));
fetchMock.catch(404);
