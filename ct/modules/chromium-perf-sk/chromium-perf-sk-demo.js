import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import { benchmarks_platforms } from './test_data';
import { pageSets } from '../pageset-selector-sk/test_data';
import { priorities } from '../task-priority-sk/test_data';
import { chromiumPatchResult } from '../patch-sk/test_data';
import 'elements-sk/error-toast-sk';

fetchMock.config.overwriteRoutes = false;
fetchMock.post('begin:/_/page_sets/', pageSets);
fetchMock.post('begin:/_/benchmarks_platforms/', benchmarks_platforms);
fetchMock.get('begin:/_/task_priorities/', priorities);
fetchMock.post('begin:/_/cl_data', chromiumPatchResult, { delay: 1000 });
// For determining running tasks for the user we just say 2.
fetchMock.postOnce('begin:/_/get', {
  data: [], ids: [], pagination: { offset: 0, size: 1, total: 2 }, permissions: [],
});
fetchMock.post('begin:/_/get', {
  data: [], ids: [], pagination: { offset: 0, size: 1, total: 0 }, permissions: [],
});

const chromiumPerf = document.createElement('chromium-perf-sk');
$$('#container').appendChild(chromiumPerf);
