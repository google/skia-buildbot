import './index';
import fetchMock from 'fetch-mock';
import { $$ } from '../../../infra-sk/modules/dom';
import { twoDemoEntries } from './test_data';

fetchMock.getOnce('/demo/metadata.json', twoDemoEntries);
const dl = document.createElement('demo-list-sk');
$$('#main')!.appendChild(dl);
