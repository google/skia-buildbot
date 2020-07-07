import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import { pageSets } from '../pageset-selector-sk/test_data';
import { buildsJson } from '../chromium-build-selector-sk/test_data';
import {
  eventPromise,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';

describe('capture-skps-sk', () => {
  fetchMock.config.overwriteRoutes = false;
  const factory = setUpElementUnderTest('capture-skps-sk');
  // Returns a new element with the pagesets, task priorirites, and
  // active tasks fetches complete, and benchmarks and platforms set.
  const newInstance = async (activeTasks, init) => {
    mockActiveTasks(activeTasks);
    fetchMock.postOnce('begin:/_/page_sets/', pageSets);
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
  let metricsAnalysis;

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

  const setDescription = (d) => {
    $$('#description', metricsAnalysis).value = d;
  };

  const clickSubmit = () => {
    $$('#submit', metricsAnalysis).click();
  };

  const expectTaskTriggered = () => {
    fetchMock.postOnce('begin:/_/add_capture_skps_task', {});
  };

  it('loads, has defaults set', async () => {
    metricsAnalysis = await newInstance();
    expect($$('#repeat_after_days', this)).to.have.property('frequency', '0');
    expect($$('#pageset_selector', this)).to.have.property('selected', '100k');
    expect($$('#chromium_build', this)).to.to.have.nested.property('build.DatastoreKey', 'foobaz');
  });

  it('requires description', async () => {
    metricsAnalysis = await newInstance();
    const event = eventPromise('error-sk');
    clickSubmit();
    const err = await event;
    expect(err.detail.message).to.equal('Please specify a description');
  });

  it('triggers a new task', async () => {
    metricsAnalysis = await newInstance();
    // Karma can't handlje page reloads, so disable it.
    metricsAnalysis._gotoRunsHistory = () => { };
    setDescription('testing');
    expectTaskTriggered();
    clickSubmit();
    $('#confirm_dialog button')[1].click(); // brittle way to press 'ok'
    await fetchMock.flush(true);
    const taskJson = JSON.parse(fetchMock.lastOptions().body);
    // Here we test the 'interesting' arguments. We try a single patch,
    // and we don't bother filling in the simple string arguments.
    const expectation = {
      chromium_build: buildsJson.data[0],
      desc: 'testing',
      page_sets: '100k',
      repeat_after_days: '0',
    };

    expect(taskJson).to.deep.equal(expectation);
  });

  it('rejects if too many active tasks', async () => {
    // Report user as having 4 active tasks.
    metricsAnalysis = await newInstance(4);
    setDescription('Testing things');
    const event = eventPromise('error-sk');
    clickSubmit();
    const err = await event;
    expect(err.detail.message).to.contain('You have 4 currently running tasks');
  });
});
