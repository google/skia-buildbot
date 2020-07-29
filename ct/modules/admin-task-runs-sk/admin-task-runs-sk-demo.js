import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import {
  tasksResult0, tasksResult1, tasksResult2,
} from './test_data';

let i = 0;
fetchMock.post('begin:/_/get_recreate_page_sets_tasks',
  () => [tasksResult0, tasksResult1, tasksResult2][i++ % 3]);
fetchMock.post('begin:/_/delete_recreate_page_sets_task', 200);
fetchMock.post('begin:/_/redo_recreate_page_sets_task', 200);
const cpr = document.createElement('admin-task-runs-sk');
cpr.taskType = 'RecreatePageSets';
cpr.getUrl = '/_/get_recreate_page_sets_tasks';
cpr.deleteUrl = '/_/delete_recreate_page_sets_task';
cpr.redoUrl = '/_/redo_recreate_page_sets_task';
$$('#container').appendChild(cpr);
