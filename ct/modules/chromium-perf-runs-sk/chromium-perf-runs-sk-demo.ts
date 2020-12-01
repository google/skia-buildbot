import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';
import fetchMock from 'fetch-mock';
import {
  tasksResult0, tasksResult1, tasksResult2,
} from './test_data';

let i = 0;
fetchMock.post('begin:/_/get_chromium_perf_tasks',
  () => [tasksResult0, tasksResult1, tasksResult2][i++ % 3]);
fetchMock.post('begin:/_/delete_chromium_perf_task', 200);
fetchMock.post('begin:/_/redo_chromium_perf_task', 200);
const cpr = document.createElement('chromium-perf-runs-sk');
($$('#container') as HTMLElement).appendChild(cpr);
