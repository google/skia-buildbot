import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import fetchMock from 'fetch-mock';
import { AdminTaskRunsSk } from './admin-task-runs-sk';
import {
  tasksResult0, tasksResult1, tasksResult2,
} from './test_data';

let i = 0;
fetchMock.post('begin:/_/get_recreate_page_sets_tasks',
  () => [tasksResult0, tasksResult1, tasksResult2][i++ % 3]);
fetchMock.post('begin:/_/delete_recreate_page_sets_task', 200);
fetchMock.post('begin:/_/redo_recreate_page_sets_task', 200);

customElements.whenDefined('admin-task-runs-sk').then(() => {
  // Insert the element later, which should given enough time for fetchMock to be in place.
    // Insert the element later, which should given enough time for fetchMock to be in place.
    document
      .querySelector('h1')!
      .insertAdjacentElement(
        'afterend',
        document.createElement('admin-task-runs-sk'),
      );

    const elems = document.querySelectorAll<AdminTaskRunsSk>('admin-task-runs-sk')!;
    elems.forEach((el) => {
      el.taskType = 'RecreatePageSets';
      el.getUrl = '/_/get_recreate_page_sets_tasks';
      el.deleteUrl = '/_/delete_recreate_page_sets_task';
      el.redoUrl = '/_/redo_recreate_page_sets_task';
    });
});
