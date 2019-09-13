import './index.js'
import '../gold-scaffold-sk'

import { delay } from '../demo_util'
import { changelistSummaries_5 } from './test_data'

(function(){

const fetchMock = require('fetch-mock');

fetchMock.get('/json/changelists', delay(changelistSummaries_5, 300));

fetchMock.catch(404);
})();