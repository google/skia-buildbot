import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import { pageSets } from '../pageset-selector-sk/test_data';
import { priorities } from '../task-priority-sk/test_data';
import { chromiumPatchResult } from '../patch-sk/test_data';

fetchMock.post('begin:/_/page_sets/', pageSets);
fetchMock.get('begin:/_/task_priorities/', priorities);
fetchMock.post('begin:/_/cl_data', chromiumPatchResult, { delay: 1000 });

function newTaskQueue(parentSelector) {
  const tq = document.createElement('chromium-perf-sk');
  $$(parentSelector).appendChild(tq);
}

newTaskQueue('#chromium-perf-container');
