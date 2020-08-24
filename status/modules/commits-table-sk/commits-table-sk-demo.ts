import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';
import { incrementalResponse0 } from '../commits-data-sk/test_data';
import fetchMock from 'fetch-mock';

fetchMock.getOnce('begin:/json/skia/incremental', incrementalResponse0)

const taskRepeater = document.createElement('commits-table-sk');
($$('#container') as HTMLElement).appendChild(taskRepeater);
