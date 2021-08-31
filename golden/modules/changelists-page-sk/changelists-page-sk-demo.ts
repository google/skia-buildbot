import './index';
import '../gold-scaffold-sk';

import fetchMock from 'fetch-mock';
import { deepCopy } from 'common-sk/modules/object';
import { delay } from '../demo_util';
import { fakeNow, changelistSummaries_5, empty } from './test_data';
import { testOnlySetSettings } from '../settings';
import { exampleStatusData } from '../last-commit-sk/demo_data';
import { GoldScaffoldSk } from '../gold-scaffold-sk/gold-scaffold-sk';

testOnlySetSettings({
  title: 'Skia Public',
  baseRepoURL: 'https://skia.googlesource.com/skia.git',
});

Date.now = () => fakeNow;

const ten = deepCopy(changelistSummaries_5);
ten.changelists!.push(...ten.changelists!);
ten.offset = 0;
ten.size = 10;
ten.total = 2147483647;

changelistSummaries_5.offset = 10;
changelistSummaries_5.size = 10;
changelistSummaries_5.total = 15;

const open = deepCopy(changelistSummaries_5);
open.changelists = open.changelists!.slice(0, 3);
open.offset = 0;
open.size = 3;
open.total = 3;

const fakeRpcDelayMillis = 300;

fetchMock.get('/json/v2/changelists?offset=0&size=10', delay(ten, fakeRpcDelayMillis));
fetchMock.get(
  '/json/v2/changelists?offset=0&size=10&active=true', delay(open, fakeRpcDelayMillis),
);
fetchMock.get(
  '/json/v2/changelists?offset=10&size=10', delay(changelistSummaries_5, fakeRpcDelayMillis),
);
fetchMock.get('glob:/json/v2/changelists*', delay(empty(), fakeRpcDelayMillis));
fetchMock.get('/json/v2/trstatus', JSON.stringify(exampleStatusData));

// By adding these elements after all the fetches are mocked out, they should load ok.
const newScaf = new GoldScaffoldSk();
newScaf.testingOffline = true;
// Make it the first element in body.
document.body.insertBefore(newScaf, document.body.childNodes[0]);
const page = document.createElement('changelists-page-sk');
page.setAttribute('page_size', '10');
newScaf.appendChild(page);
