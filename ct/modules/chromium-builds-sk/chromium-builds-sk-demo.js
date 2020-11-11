import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import { chromiumRevResult, skiaRevResult } from './test_data';
import 'elements-sk/error-toast-sk';

fetchMock.config.overwriteRoutes = false;
fetchMock.post('begin:/_/chromium_rev_data', chromiumRevResult, { delay: 1000 });
fetchMock.post('begin:/_/skia_rev_data', skiaRevResult, { delay: 1000 });
// For determining running tasks for the user we just say 2.
fetchMock.postOnce('begin:/_/get', {
  data: [], ids: [], pagination: { offset: 0, size: 1, total: 2 }, permissions: [],
});
fetchMock.post('begin:/_/get', {
  data: [], ids: [], pagination: { offset: 0, size: 1, total: 0 }, permissions: [],
});

const chromiumPerf = document.createElement('chromium-builds-sk');
$$('#container').appendChild(chromiumPerf);
