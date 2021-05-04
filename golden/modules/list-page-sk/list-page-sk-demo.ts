import './index';
import '../gold-scaffold-sk';
import { delay } from '../demo_util';
import { manyParams } from '../shared_demo_data';
import { testOnlySetSettings } from '../settings';
import { sampleByTestList } from './test_data';
import { exampleStatusData } from '../last-commit-sk/demo_data';
import fetchMock from 'fetch-mock';
import { ListPageSk } from './list-page-sk';
import { GoldScaffoldSk } from '../gold-scaffold-sk/gold-scaffold-sk';

testOnlySetSettings({
  title: 'Testing Gold',
  defaultCorpus: 'gm',
  baseRepoURL: 'https://github.com/flutter/flutter',
});

fetchMock.get('/json/v1/paramset', delay(manyParams, 100));
fetchMock.get('glob:/json/v1/list*', delay(sampleByTestList, 100));
fetchMock.get('/json/v1/trstatus', JSON.stringify(exampleStatusData));

// By adding these elements after all the fetches are mocked out, they should load ok.
const newScaf = new GoldScaffoldSk();
newScaf.testingOffline = true;
// Make it the first element in body.
document.body.insertBefore(newScaf, document.body.childNodes[0]);
newScaf.appendChild(new ListPageSk());
