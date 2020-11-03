import './index';
import { expect } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import { StatusSk } from './status-sk';

import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { AlertsStatus } from '../../../perf/modules/json/index';
import { incrementalResponse0, SetupMocks } from '../rpc-mock';
import fetchMock from 'fetch-mock';
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

describe('status-sk', () => {
  const newInstance = setUpElementUnderTest<StatusSk>('status-sk');

  let element: StatusSk;
  beforeEach(async () => {
    SetTestSettings({
      swarmingUrl: 'example.com/swarming',
      logsUrlTemplate:
        'https://ci.chromium.org/raw/build/logs.chromium.org/skia/TASKID/+/annotations',
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
    fetchMock.getOnce('https://tree-status.skia.org/current', treeStatusResp);
    fetchMock.getOnce('https://tree-status.skia.org/current-sheriff', generalRoleResp);
    fetchMock.getOnce('https://tree-status.skia.org/current-wrangler', gpuRoleResp);
    fetchMock.getOnce('https://tree-status.skia.org/current-robocop', androidRoleResp);
    fetchMock.getOnce('https://tree-status.skia.org/current-trooper', infraRoleResp);
    Date.now = () => 1600883976659;
    SetupMocks().expectGetIncrementalCommits(incrementalResponse0);
    const ep = eventPromise('end-task');
    element = newInstance();
    await ep;
  });

  it('reacts to repo-changed', async () => {
    expect($$('h1', element)).to.have.property('innerText', 'Status: Skia');
    const repoSelector = $$('#repoSelector', element) as HTMLSelectElement;
    repoSelector.value = 'infra';
    const ep = eventPromise('end-task');
    repoSelector.dispatchEvent(new Event('change', { bubbles: true }));
    await ep;
    expect($$('h1', element)).to.have.property('innerText', 'Status: Infra');
  });
});
