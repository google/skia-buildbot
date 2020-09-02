import './index';
import '../gold-scaffold-sk';

import { fetchMock } from 'fetch-mock';
import { deepCopy } from 'common-sk/modules/object';
import { $$ } from 'common-sk/modules/dom';
import { delay } from '../demo_util';
import { fakeNow, changelistSummaries_5, empty } from './test_data';
import { testOnlySetSettings } from '../settings';
import { exampleStatusData } from '../last-commit-sk/demo_data';

testOnlySetSettings({
  title: 'Skia Public',
  baseRepoURL: 'https://skia.googlesource.com/skia.git',
});

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
fetchMock.get('/json/trstatus', JSON.stringify(exampleStatusData));

// By adding these elements after all the fetches are mocked out, they should load ok.
const newScaf = document.createElement('gold-scaffold-sk');
newScaf.setAttribute('testing_offline', 'true');
const body = $$('body');
body.insertBefore(newScaf, body.childNodes[0]); // Make it the first element in body.
const page = document.createElement('changelists-page-sk');
page.setAttribute('page_size', '10');
newScaf.appendChild(page);
