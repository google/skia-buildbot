import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';
import fetchMock from 'fetch-mock';
import {
  singleResultCanDelete, singleResultNoDelete, resultSetTwoItems,
} from './test_data';

function newTaskQueue(parentSelector: string) {
  fetchMock.config.overwriteRoutes = false;
  fetchMock.postOnce('begin:/_/get_', resultSetTwoItems);
  fetchMock.postOnce('begin:/_/get_', singleResultCanDelete);
  fetchMock.postOnce('begin:/_/get_', singleResultNoDelete);
  fetchMock.post('begin:/_/get_', 200, { repeat: 13 });
  const tq = document.createElement('task-queue-sk');
  ($$(parentSelector) as HTMLElement).appendChild(tq);
}

newTaskQueue('#task-queue-container');
