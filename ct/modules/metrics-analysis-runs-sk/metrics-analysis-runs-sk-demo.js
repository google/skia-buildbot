import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import {
  tasksResult0, tasksResult1, tasksResult2,
} from './test_data';

let i = 0;
fetchMock.post('begin:/_/get_metrics_analysis_tasks',
  () => [tasksResult0, tasksResult1, tasksResult2][i++ % 3]);
fetchMock.post('begin:/_/delete_metrics_analysis_task', 200);
fetchMock.post('begin:/_/redo_metrics_analysis_task', 200);
const cpr = document.createElement('metrics-analysis-runs-sk');
$$('#container').appendChild(cpr);
