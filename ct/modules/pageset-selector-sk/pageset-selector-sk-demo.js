import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import { pageSets } from './test_data';

fetchMock.post('begin:/_/page_sets/', pageSets);
const pageSetSelector = document.createElement('pageset-selector-sk');
$$('#container').appendChild(pageSetSelector);
