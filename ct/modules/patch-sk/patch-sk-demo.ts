import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import fetchMock from 'fetch-mock';
import { $$ } from '../../../infra-sk/modules/dom';
import { chromiumPatchResult } from './test_data';
import { PatchSk } from './patch-sk';

fetchMock.config.overwriteRoutes = false;
fetchMock.postOnce('begin:/_/cl_data', chromiumPatchResult, { delay: 1000 });
fetchMock.post('*', 503);
const tq = document.createElement('patch-sk') as PatchSk;
tq.patchType = 'chromium';
($$('#container') as HTMLElement).appendChild(tq);
