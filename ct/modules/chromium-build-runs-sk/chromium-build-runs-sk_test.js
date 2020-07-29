import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';

import {
  tasksResult0, tasksResult1,
} from './test_data';
import {
  eventPromise,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';

describe('chromium-build-runs-sk', () => {
  const newInstance = setUpElementUnderTest('chromium-build-runs-sk');
  fetchMock.config.overwriteRoutes = false;

  let analysisRuns;
  beforeEach(async () => {
    await expectReload(() => analysisRuns = newInstance());
  });

  afterEach(() => {
    //  Check all mock fetches called at least once and reset.
    expect(fetchMock.done()).to.be.true;
    fetchMock.reset();
  });

  // Expect 'trigger' to cause a reload, and execute it.
  // Optionally pass desired result from server.
  const expectReload = async (trigger, result) => {
    result = result || tasksResult0;
    const event = eventPromise('end-task');
    fetchMock.postOnce('begin:/_/get_chromium_build_tasks', result);
    trigger();
    await event;
  };

  const confirmDialog = () => $$('dialog', analysisRuns).querySelectorAll('button')[1].click();

  it('shows table entries', async () => {
    expect($('table.runssummary>tbody>tr', analysisRuns)).to.have.length(6);
    expect(fetchMock.lastUrl()).to.contain('offset=0');
    expect(fetchMock.lastUrl()).to.contain('size=10');
  });

  it('navigates with pages', async () => {
    expect(fetchMock.lastUrl()).to.contain('offset=0');
    const result = tasksResult1;
    result.pagination.offset = 10;
    // 'Next page' button.
    await expectReload(
      () => $('pagination-sk button.action', analysisRuns)[2].click(), result);
    expect(fetchMock.lastUrl()).to.contain('offset=10');
    expect($('table.runssummary>tbody>tr', analysisRuns)).to.have.length(5);
  });

  it('deletes tasks', async () => {
    $$('delete-icon-sk', analysisRuns).click();
    fetchMock.post('begin:/_/delete_chromium_build_task', 200);
    await expectReload(confirmDialog);
    expect(fetchMock.lastOptions('begin:/_/delete').body).to.contain('"id":23');
  });

  it('reschedules tasks', async () => {
    $$('redo-icon-sk', analysisRuns).click();
    fetchMock.post('begin:/_/redo_chromium_build_task', 200);
    await expectReload(confirmDialog);
    expect(fetchMock.lastOptions('begin:/_/redo').body).to.contain('"id":23');
  });
});
