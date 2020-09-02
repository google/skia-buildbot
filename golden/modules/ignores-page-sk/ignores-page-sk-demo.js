import './index';
import '../gold-scaffold-sk';

import { $$ } from 'common-sk/modules/dom';
import { delay } from '../demo_util';
import { ignoreRules_10, fakeNow } from './test_data';
import { manyParams } from '../shared_demo_data';
import { testOnlySetSettings } from '../settings';
import { exampleStatusData } from '../last-commit-sk/demo_data';

const fetchMock = require('fetch-mock');

Date.now = () => fakeNow;
testOnlySetSettings({
  title: 'Skia Public',
  baseRepoURL: 'https://skia.googlesource.com/skia.git',
});

fetchMock.get('/json/paramset', delay(manyParams, 100));
fetchMock.get('/json/ignores?counts=1', delay(ignoreRules_10, 300));
fetchMock.post('glob:/json/ignores/del/*', delay({}, 600));
fetchMock.post('glob:/json/ignores/add/', delay({}, 600));
fetchMock.post('glob:/json/ignores/save/*', delay({}, 600));
fetchMock.get('/json/trstatus', JSON.stringify(exampleStatusData));

// By adding these elements after all the fetches are mocked out, they should load ok.
const newScaf = document.createElement('gold-scaffold-sk');
newScaf.setAttribute('testing_offline', 'true');
const body = $$('body');
body.insertBefore(newScaf, body.childNodes[0]); // Make it the first element in body.
const page = document.createElement('ignores-page-sk');
page.setAttribute('page_size', '10');
newScaf.appendChild(page);
