import './index';
import { StatusSk } from './status-sk';

import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';
import { $, $$ } from 'common-sk/modules/dom';
import { incrementalResponse0, SetupMocks } from '../rpc-mock';
import fetchMock from 'fetch-mock';
import { SetTestSettings } from '../settings';

describe('status-sk', () => {
  const newInstance = setUpElementUnderTest<StatusSk>('status-sk');

  let element: StatusSk;
  beforeEach(async () => {
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
