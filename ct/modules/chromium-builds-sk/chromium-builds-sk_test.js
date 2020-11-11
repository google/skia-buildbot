import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import { chromiumRevResult, skiaRevResult } from './test_data';
import { buildsJson } from '../chromium-build-selector-sk/test_data';
import {
  eventPromise,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';

describe('chromium-builds-sk', () => {
  fetchMock.config.overwriteRoutes = false;
  const factory = setUpElementUnderTest('chromium-builds-sk');
  // Returns a new element with the pagesets, task priorirites, and
  // active tasks fetches complete, and benchmarks and platforms set.
  const newInstance = async (activeTasks) => {
    mockActiveTasks(activeTasks);
    // Create a unique dummy result to distinguish from when we load
    // non-LKGR revisions.
    const lkgrDummy = Object.assign({}, chromiumRevResult);
    lkgrDummy.commit = 'aaaaaa';
    fetchMock.postOnce('begin:/_/chromium_rev_data?rev=LKGR', lkgrDummy);
    fetchMock.postOnce('begin:/_/skia_rev_data?rev=LKGR', lkgrDummy);
    const ele = factory();
    await fetchMock.flush(true);
    return ele;
  };

  // Make our test object global to make helper functions convenient.
  let chromiumBuilds;

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
    $$('#submit', chromiumBuilds).click();
  };

  const setChromiumRev = async (value, result) => {
    if (result) {
      fetchMock.postOnce('begin:/_/chromium_rev_data', result);
    }
    const input = $$('#chromium_rev');
    input.value = value;
    input.dispatchEvent(new Event('input', {
      bubbles: true,
    }));
    await fetchMock.flush(true);
  };

  const setSkiaRev = async (value, result) => {
    if (result) {
      fetchMock.postOnce('begin:/_/skia_rev_data', result);
    }
    const input = $$('#skia_rev');
    input.value = value;
    input.dispatchEvent(new Event('input', {
      bubbles: true,
    }));
    await fetchMock.flush(true);
  };

  it('loads, has defaults set', async () => {
    chromiumBuilds = await newInstance();
    expect($$('#repeat_after_days', this)).to.have.property('frequency', '0');
    expect($$('#chromium_rev', this)).to.have.property('value', 'LKGR');
    expect($$('#skia_rev', this)).to.have.property('value', 'LKGR');
  });

  it('requires Chromium hash', async () => {
    chromiumBuilds = await newInstance();
    // Empty input is invalid.
    await setChromiumRev('');
    let event = eventPromise('error-sk');
    clickSubmit();
    let err = await event;
    expect(err.detail.message).to.contain('Please enter a valid Chromium commit hash.');
    // Backend doesn't give a valid revision.
    await setChromiumRev('abc', 503);
    event = eventPromise('error-sk');
    clickSubmit();
    err = await event;
    expect(err.detail.message).to.contain('Please enter a valid Chromium commit hash.');
  });

  it('requires Skia hash', async () => {
    chromiumBuilds = await newInstance();
    // Empty input is invalid.
    await setSkiaRev('');
    let event = eventPromise('error-sk');
    clickSubmit();
    let err = await event;
    expect(err.detail.message).to.contain('Please enter a valid Skia commit hash.');
    // Backend doesn't give a valid revision.
    await setSkiaRev('abc', 503);
    event = eventPromise('error-sk');
    clickSubmit();
    err = await event;
    expect(err.detail.message).to.contain('Please enter a valid Skia commit hash.');
  });

  it('triggers a new task, LKGR', async () => {
    chromiumBuilds = await newInstance();
    // Karma can't handlje page reloads, so disable it.
    chromiumBuilds._gotoRunsHistory = () => { };
    clickSubmit();
    fetchMock.postOnce('begin:/_/add_chromium_build_task', {});
    $('#confirm_dialog button')[1].click(); // brittle way to press 'ok'
    await fetchMock.flush(true);
    const taskJson = JSON.parse(fetchMock.lastOptions().body);
    const expectation = {
      chromium_rev: 'aaaaaa',
      chromium_rev_ts: 'Mon May 08 21:08:33 2017',
      skia_rev: 'aaaaaa',
      repeat_after_days: '0',
    };

    expect(taskJson).to.deep.equal(expectation);
  });

  it('triggers a new task, explicit', async () => {
    chromiumBuilds = await newInstance();
    // Karma can't handlje page reloads, so disable it.
    chromiumBuilds._gotoRunsHistory = () => { };
    await setChromiumRev('abc', chromiumRevResult);
    await setSkiaRev('abc', skiaRevResult);
    clickSubmit();
    fetchMock.postOnce('begin:/_/add_chromium_build_task', {});
    $('#confirm_dialog button')[1].click(); // brittle way to press 'ok'
    await fetchMock.flush(true);
    const taskJson = JSON.parse(fetchMock.lastOptions().body);
    const expectation = {
      chromium_rev: 'deadbeefdeadbeef',
      chromium_rev_ts: 'Mon May 08 21:08:33 2017',
      skia_rev: '123456789abcdef',
      repeat_after_days: '0',
    };

    console.log(taskJson);
    console.log(expectation);
    expect(taskJson).to.deep.equal(expectation);
  });

  it('rejects if too many active tasks', async () => {
    // Report user as having 4 active tasks.
    chromiumBuilds = await newInstance(4);
    const event = eventPromise('error-sk');
    clickSubmit();
    const err = await event;
    expect(err.detail.message).to.contain('You have 4 currently running tasks');
  });
});
