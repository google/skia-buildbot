import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import fetchMock from 'fetch-mock';
import { $$ } from '../../../infra-sk/modules/dom';
import { pageSets } from './test_data';

fetchMock.post('begin:/_/page_sets/', pageSets);
const pageSetSelector = document.createElement('pageset-selector-sk');
($$('#container') as HTMLElement).appendChild(pageSetSelector);
