import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import { priorities } from '../task-priority-sk/test_data';
import { chromiumPatchResult } from '../patch-sk/test_data';
import 'elements-sk/error-toast-sk';

fetchMock.config.overwriteRoutes = false;
fetchMock.get('begin:/_/task_priorities/', priorities);
fetchMock.post('begin:/_/cl_data', chromiumPatchResult, { delay: 1000 });
// For determining running tasks for the user we just say 2.
fetchMock.postOnce('begin:/_/get', {
  data: [], ids: [], pagination: { offset: 0, size: 1, total: 2 }, permissions: [],
});
fetchMock.post('begin:/_/get', {
  data: [], ids: [], pagination: { offset: 0, size: 1, total: 0 }, permissions: [],
});

const chromiumPerf = document.createElement('metrics-analysis-sk');
$$('#container').appendChild(chromiumPerf);
