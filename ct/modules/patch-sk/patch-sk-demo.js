import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import {
  chromiumPatchResult
} from './test_data';

fetchMock.config.overwriteRoutes = false;
fetchMock.postOnce('begin:/_/cl_data', chromiumPatchResult, { delay: 1000 });
fetchMock.post('*', 503);
const tq = document.createElement('patch-sk');
tq.patchType = 'chromium';
$$('#container').appendChild(tq);
