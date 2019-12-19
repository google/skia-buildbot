import './index.js'
import '../gold-scaffold-sk'

import { delay } from '../demo_util'
import { changelistSummaries_5, empty } from './test_data'

(function(){

const fetchMock = require('fetch-mock');

const ten = JSON.parse(JSON.stringify(changelistSummaries_5));
ten.data.push(...ten.data);
ten.pagination = {
  offset: 0,
  size: 10,
  total: 2147483647,
};

changelistSummaries_5.pagination = {
  offset: 10,
  size: 10,
  total: 15,
};

const open = JSON.parse(JSON.stringify(changelistSummaries_5));
open.data = open.data.slice(0, 3);
open.pagination = {
  offset: 0,
  size: 3,
  total: 3,
};

fetchMock.get('/json/changelists?offset=0&size=10', delay(ten, 300));
fetchMock.get('/json/changelists?offset=0&size=10&active=true', delay(open, 300));
fetchMock.get('/json/changelists?offset=10&size=10', delay(changelistSummaries_5, 300));
fetchMock.get('glob:/json/changelists*', delay(empty, 300));

fetchMock.catch(404);
})();
