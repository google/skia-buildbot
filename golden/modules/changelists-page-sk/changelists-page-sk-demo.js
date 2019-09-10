import './index.js'
import '../gold-scaffold-sk'

import { changelistSummaries_5 } from './test_data'

(function(){

const fetchMock = require('fetch-mock');

fetchMock.get('/json/changelists', changelistSummaries_5);

})();