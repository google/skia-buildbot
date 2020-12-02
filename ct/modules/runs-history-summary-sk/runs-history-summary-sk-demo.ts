import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import {
  summaryResults3, summaryResults5, summaryResults15, summaryResults33
} from './test_data';

fetchMock.post('begin:/_/completed_tasks', () => {
  // Cheat so we don't have to compute timestamps to determine the period.
  switch($$('runs-history-summary-sk').period) {
    case 7:
      return summaryResults3;
    case 30:
      return summaryResults5;
    case 365:
      return summaryResults15
    default:
      return summaryResults33;
  }
});
const rs = document.createElement('runs-history-summary-sk');
$$('#container').appendChild(rs);
