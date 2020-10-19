import './index';
import { getAutorollerStatusesResponse, incrementalResponse0, SetupMocks } from '../rpc-mock';
import { AlertsStatus } from '../../../perf/modules/json/index';
import { $$ } from 'common-sk/modules/dom';
import { SetTestSettings } from '../settings';
import fetchMock from 'fetch-mock';

SetupMocks()
  .expectGetIncrementalCommits(incrementalResponse0)
  .expectGetAutorollerStatuses(getAutorollerStatusesResponse);
SetTestSettings({
  swarmingUrl: 'example.com/swarming',
  taskSchedulerUrl: 'example.com/ts',
  defaultRepo: 'skia',
  repos: new Map([
    ['skia', 'https://skia.googlesource.com/skia/+show/'],
    ['infra', 'https://skia.googlesource.com/buildbot/+show/'],
    ['skcms', 'https://skia.googlesource.com/skcms/+show/'],
  ]),
});
fetchMock.getOnce('https://perf.skia.org/_/alerts/', <AlertsStatus>{ alerts: 5 });
const data = document.createElement('status-sk');
($$('#container') as HTMLElement).appendChild(data);
(document.querySelector('#AllFilter') as HTMLElement).click();

document.querySelector('status-sk')!.addEventListener('some-event-name', (e) => {
  document.querySelector('#events')!.textContent = JSON.stringify(e, null, '  ');
});
