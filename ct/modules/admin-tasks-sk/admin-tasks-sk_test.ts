import './index';

import sinon from 'sinon';
import { $, $$ } from 'common-sk/modules/dom';
import fetchMock from 'fetch-mock';
import { expect } from 'chai';
import { pageSets } from '../pageset-selector-sk/test_data';
import { AdminTasksSk } from './admin-tasks-sk';
import {
  eventPromise,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';

describe('admin-tasks-sk', () => {
  fetchMock.config.overwriteRoutes = false;
  const factory = setUpElementUnderTest<AdminTasksSk>('admin-tasks-sk');
  // Returns a new element with the pagesets, task priorirites, and
  // active tasks fetches complete, and benchmarks and platforms set.
  const newInstance = async (activeTasks?: number) => {
    mockActiveTasks(activeTasks);
    fetchMock.post('begin:/_/page_sets/', pageSets, { repeat: 2 });
    const ele = factory();
    await fetchMock.flush(true);
    return ele;
  };

  // Make our test object global to make helper functions convenient.
  let adminTasks: AdminTasksSk;

  afterEach(() => {
    //  Check all mock fetches called at least once and reset.
    expect(fetchMock.done()).to.be.true;
    fetchMock.reset();
    sinon.restore();
  });

  const mockActiveTasks = (n: number|undefined) => {
    n = n || 0;
    // For running tasks for the user we put a nonzero total in one of the
    // responses, and 0 in the remaining 6.
    fetchMock.postOnce('begin:/_/get', {
      data: [],
      ids: [],
      pagination: { offset: 0, size: 1, total: n },
      permissions: [],
    });
    fetchMock.post('begin:/_/get', {
      data: [],
      ids: [],
      pagination: { offset: 0, size: 1, total: 0 },
      permissions: [],
    }, { repeat: 6 });
  };

  const clickSubmit = () => {
    ($$('#submit', adminTasks)! as HTMLElement).click();
  };

  it('loads, default tab', async () => {
    adminTasks = await newInstance();
    expect($('#repeat_after_days', adminTasks)).to.have.length(2);
    expect($('#pageset_selector', adminTasks)).to.have.length(2);
    expect(adminTasks._activeTab).to.have.property('id', 'pagesets');
  });

  it('triggers a new pagesets task', async () => {
    adminTasks = await newInstance();
    adminTasks._gotoRunsHistory = () => {
      // Karma can't handlje page reloads, so disable it.
    };
    fetchMock.postOnce('begin:/_/add_recreate_page_sets_task', {});
    sinon.stub(window, 'confirm').returns(true);
    clickSubmit();
    await fetchMock.flush(true);
    const taskJson = JSON.parse(fetchMock.lastOptions()!.body as any);
    const expectation = {
      page_sets: '10k',
      repeat_after_days: '0',
    };

    expect(taskJson).to.deep.equal(expectation);
  });

  it('triggers a new archives task', async () => {
    adminTasks = await newInstance();
    adminTasks._gotoRunsHistory = () => {
      // Karma can't handlje page reloads, so disable it.
    };
    fetchMock.postOnce('begin:/_/add_recreate_webpage_archives_task', {});
    sinon.stub(window, 'confirm').returns(true);
    // Click archives tab.
    ($('tabs-sk button', adminTasks)[1] as HTMLElement).click();
    clickSubmit();
    await fetchMock.flush(true);
    const taskJson = JSON.parse(fetchMock.lastOptions()!.body as any);
    const expectation = {
      page_sets: '10k',
      repeat_after_days: '0',
    };

    expect(taskJson).to.deep.equal(expectation);
  });

  it('rejects if too many active tasks', async () => {
    // Report user as having 4 active tasks.
    adminTasks = await newInstance(4);
    const event = eventPromise('error-sk');
    clickSubmit();
    const err = await event;
    expect((err as CustomEvent).detail.message).to.contain('You have 4 currently running tasks');
  });
});
