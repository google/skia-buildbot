import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import { pageSets } from '../pageset-selector-sk/test_data';
import { buildsJson } from '../chromium-build-selector-sk/test_data';
import {
  eventPromise,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('admin-tasks-sk', () => {
  fetchMock.config.overwriteRoutes = false;
  const factory = setUpElementUnderTest('admin-tasks-sk');
  // Returns a new element with the pagesets, task priorirites, and
  // active tasks fetches complete, and benchmarks and platforms set.
  const newInstance = async (activeTasks, init) => {
    mockActiveTasks(activeTasks);
    fetchMock.post('begin:/_/page_sets/', pageSets, { repeat: 2 });
    fetchMock.postOnce('begin:/_/get_chromium_build_tasks', buildsJson);

    const wrappedInit = (ele) => {
      if (init) {
        init(ele);
      }
    };
    const ele = factory(wrappedInit);
    await fetchMock.flush(true);
    return ele;
  };

  // Make our test object global to make helper functions convenient.
  let adminTasks;

  afterEach(() => {
    //  Check all mock fetches called at least once and reset.
    expect(fetchMock.done()).to.be.true;
    fetchMock.reset();
  });

  const mockActiveTasks = (n) => {
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
    $$('#submit', adminTasks).click();
  };

  it('loads, default tab', async () => {
    adminTasks = await newInstance();
    expect($('#repeat_after_days', adminTasks)).to.have.length(2);
    expect($('#pageset_selector', adminTasks)).to.have.length(2);
    expect($('#chromium_build', adminTasks)).to.have.length(1);
    expect(adminTasks._activeTab).to.have.property('id', 'pagesets');
  });

  it('triggers a new pagesets task', async () => {
    adminTasks = await newInstance();
    // Karma can't handlje page reloads, so disable it.
    adminTasks._gotoRunsHistory = () => { };
    fetchMock.postOnce('begin:/_/add_recreate_page_sets_task', {});
    clickSubmit();
    $('#confirm_dialog button')[1].click(); // brittle way to press 'ok'
    await fetchMock.flush(true);
    const taskJson = JSON.parse(fetchMock.lastOptions().body);
    const expectation = {
      page_sets: '100k',
      repeat_after_days: '0',
    };

    expect(taskJson).to.deep.equal(expectation);
  });

  it('triggers a new archives task', async () => {
    adminTasks = await newInstance();
    // Karma can't handlje page reloads, so disable it.
    adminTasks._gotoRunsHistory = () => { };
    fetchMock.postOnce('begin:/_/add_recreate_webpage_archives_task', {});
    // Click archives tab.
    $('tabs-sk button', adminTasks)[1].click();
    clickSubmit();
    $('#confirm_dialog button')[1].click(); // brittle way to press 'ok'
    await fetchMock.flush(true);
    const taskJson = JSON.parse(fetchMock.lastOptions().body);
    const expectation = {
      chromium_build: buildsJson.data[0],
      page_sets: '100k',
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
    expect(err.detail.message).to.contain('You have 4 currently running tasks');
  });
});
