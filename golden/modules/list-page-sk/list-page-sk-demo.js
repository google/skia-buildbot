import './index';
import '../gold-scaffold-sk';
import { $$ } from 'common-sk/modules/dom';
import { delay } from '../demo_util';
import { manyParams } from '../shared_demo_data';
import { testOnlySetSettings } from '../settings';
import { sampleByTestList } from './test_data';
import { exampleStatusData } from '../last-commit-sk/demo_data';
import fetchMock from 'fetch-mock';

testOnlySetSettings({
  title: 'Testing Gold',
  defaultCorpus: 'gm',
  baseRepoURL: 'https://github.com/flutter/flutter',
});

fetchMock.get('/json/v1/paramset', delay(manyParams, 100));
fetchMock.get('glob:/json/v1/list*', delay(sampleByTestList, 100));
fetchMock.get('/json/v1/trstatus', JSON.stringify(exampleStatusData));

// By adding these elements after all the fetches are mocked out, they should load ok.
const newScaf = document.createElement('gold-scaffold-sk');
newScaf.setAttribute('testing_offline', 'true');
const body = $$('body');
body.insertBefore(newScaf, body.childNodes[0]); // Make it the first element in body.
const page = document.createElement('list-page-sk');
newScaf.appendChild(page);
