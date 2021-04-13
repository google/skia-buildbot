import './index';
import { $$ } from 'common-sk/modules/dom';
import fetchMock from 'fetch-mock';
import {
  twoDemoEntries,
} from './test_data';

fetchMock.getOnce('/demo/metadata.json', twoDemoEntries);
const dl = document.createElement('demo-list-sk');
$$('#main').appendChild(dl);
