import './index';
import { $$ } from 'common-sk/modules/dom';
import fetchMock from 'fetch-mock';
import { getAutorollerStatusesResponse, incrementalResponse0, SetupMocks } from '../rpc-mock';
import { AlertsStatus } from '../../../perf/modules/json/index';
import { SetTestSettings } from '../settings';
import { StatusResponse } from '../../../golden/modules/rpc_types';
import { GetClientCountsResponse, StatusData } from '../../../bugs-central/modules/json';
import {
  treeStatusResp,
  generalRoleResp,
  gpuRoleResp,
  androidRoleResp,
  infraRoleResp,
} from '../tree-status-sk/test_data';

Date.now = () => Date.parse('2020-12-31T00:00:00.000Z');

SetupMocks()
  .expectGetIncrementalCommits(incrementalResponse0)
  .expectGetAutorollerStatuses(getAutorollerStatusesResponse);
SetTestSettings({
  swarmingUrl: 'example.com/swarming',
  treeStatusBaseUrl: 'https://example.com/treestatus',
  logsUrlTemplate:
    'https://ci.chromium.org/raw/build/logs.chromium.org/skia/{{TaskID}}/+/annotations',
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
fetchMock.getOnce('https://gold.skia.org/json/v2/trstatus', <StatusResponse>{
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
fetchMock.getOnce('https://bugs-central.skia.org/get_client_counts', <GetClientCountsResponse>{
  clients_to_status_data: {
    Android: <StatusData>{
      untriaged_count: 10,
      link: 'www.test-link.com/test1',
    },
    Chromium: <StatusData>{
      untriaged_count: 23,
      link: 'www.test-link.com/test2',
    },
    Skia: <StatusData>{
      untriaged_count: 104,
      link: 'www.test-link.com/test3',
    },
  },
});
fetchMock.getOnce('https://example.com/treestatus/skia/current', treeStatusResp);
fetchMock.getOnce('https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-gardener', generalRoleResp);
fetchMock.getOnce('https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-gpu-gardener', gpuRoleResp);
fetchMock.getOnce('https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-android-gardener', androidRoleResp);
fetchMock.getOnce('https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-infra-gardener', infraRoleResp);
const data = document.createElement('status-sk');
($$('#container') as HTMLElement).appendChild(data);

(document.querySelector('#AllFilter') as HTMLElement).click();

document.querySelector('status-sk')!.addEventListener('some-event-name', (e) => {
  document.querySelector('#events')!.textContent = JSON.stringify(e, null, '  ');
});
