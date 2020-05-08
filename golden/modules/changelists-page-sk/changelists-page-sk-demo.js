import './index';
import '../gold-scaffold-sk';

import { fetchMock } from 'fetch-mock';
import { deepCopy } from 'common-sk/modules/object';
import { delay } from '../demo_util';
import { fakeNow, changelistSummaries_5, empty } from './test_data';
import { testOnlySetSettings } from '../settings';
import { $$ } from 'common-sk/modules/dom';

testOnlySetSettings({
  title: 'Skia Public',
});
$$('gold-scaffold-sk')._render(); // pick up title from settings.

Date.now = () => fakeNow;

const ten = deepCopy(changelistSummaries_5);
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

const open = deepCopy(changelistSummaries_5);
open.data = open.data.slice(0, 3);
open.pagination = {
  offset: 0,
  size: 3,
  total: 3,
};

const fakeRpcDelayMillis = 300;

fetchMock.get('/json/changelists?offset=0&size=10', delay(ten, fakeRpcDelayMillis));
fetchMock.get('/json/changelists?offset=0&size=10&active=true', delay(open, fakeRpcDelayMillis));
fetchMock.get('/json/changelists?offset=10&size=10', delay(changelistSummaries_5, fakeRpcDelayMillis));
fetchMock.get('glob:/json/changelists*', delay(empty, fakeRpcDelayMillis));
