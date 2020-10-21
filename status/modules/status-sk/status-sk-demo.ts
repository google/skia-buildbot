import './index';
import { getAutorollerStatusesResponse, incrementalResponse0, SetupMocks } from '../rpc-mock';
import { AlertsStatus } from '../../../perf/modules/json/index';
import { $$ } from 'common-sk/modules/dom';
import { SetTestSettings } from '../settings';
import fetchMock from 'fetch-mock';
import { StatusResponse } from '../../../golden/modules/rpc_types';
import {
  treeStatusResp,
  generalRoleResp,
  gpuRoleResp,
  androidRoleResp,
  infraRoleResp,
} from '../tree-status-sk/test_data';

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
fetchMock.getOnce('path:/loginstatus/', {});
fetchMock.getOnce('https://perf.skia.org/_/alerts/', <AlertsStatus>{ alerts: 5 });
fetchMock.getOnce('https://gold.skia.org/json/v1/trstatus', <StatusResponse>{
  corpStatus: [
    { name: 'canvaskit', untriagedCount: 0 },
    { name: 'colorImage', untriagedCount: 0 },
    { name: 'gm', untriagedCount: 13 },
    { name: 'image', untriagedCount: 0 },
    { name: 'pathkit', untriagedCount: 0 },
    { name: 'skp', untriagedCount: 0 },
    { name: 'svg', untriagedCount: 27 },
  ],
});
fetchMock.getOnce('https://tree-status.skia.org/current', treeStatusResp);
fetchMock.getOnce('https://tree-status.skia.org/current-sheriff', generalRoleResp);
fetchMock.getOnce('https://tree-status.skia.org/current-wrangler', gpuRoleResp);
fetchMock.getOnce('https://tree-status.skia.org/current-robocop', androidRoleResp);
fetchMock.getOnce('https://tree-status.skia.org/current-trooper', infraRoleResp);
Date.now = () => 1600883976659;
const data = document.createElement('status-sk');
($$('#container') as HTMLElement).appendChild(data);
(document.querySelector('#AllFilter') as HTMLElement).click();

document.querySelector('status-sk')!.addEventListener('some-event-name', (e) => {
  document.querySelector('#events')!.textContent = JSON.stringify(e, null, '  ');
});
