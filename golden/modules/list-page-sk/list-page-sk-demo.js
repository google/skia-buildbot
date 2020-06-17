import './index';
import '../gold-scaffold-sk';
import { $$ } from 'common-sk/modules/dom';
import { delay } from '../demo_util';
import { manyParams } from '../shared_demo_data';
import { testOnlySetSettings } from '../settings';
import { sampleByTestList } from './test_data';

testOnlySetSettings({
  title: 'Testing Gold',
  defaultCorpus: 'gm',
});

const fetchMock = require('fetch-mock');

fetchMock.get('/json/paramset', delay(manyParams, 100));
fetchMock.get('glob:/json/list2*', delay(sampleByTestList, 100));

// By adding this element after all the fetches are mocked out, it should load ok.
const newList = document.createElement('list-page-sk');
const scaffold = $$('gold-scaffold-sk');
scaffold._render(); // update Title
$$('gold-scaffold-sk').appendChild(newList);
