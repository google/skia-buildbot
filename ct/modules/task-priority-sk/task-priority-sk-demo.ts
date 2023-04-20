import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import fetchMock from 'fetch-mock';
import { $$ } from '../../../infra-sk/modules/dom';
import { priorities } from './test_data';

fetchMock.get('begin:/_/task_priorities/', priorities);
const pageSetSelector = document.createElement('task-priority-sk');
($$('#container') as HTMLElement).appendChild(pageSetSelector);
