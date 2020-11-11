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

describe('admin-task-runs-sk', () => {
  const newInstance = setUpElementUnderTest('admin-task-runs-sk');
  const taskType = 'RecreatePageSets';
  const getUrl = '/_/get_recreate_page_sets_tasks';
  const deleteUrl = '/_/delete_recreate_page_sets_task';
  const redoUrl = '/_/redo_recreate_page_sets_task';

  fetchMock.config.overwriteRoutes = false;

  let captureSkpRuns;
  beforeEach(async () => {
    await expectReload(() => captureSkpRuns = newInstance(
      (el) => {
        el.taskType = taskType;
        el.getUrl = getUrl;
        el.deleteUrl = deleteUrl;
        el.redoUrl = redoUrl;
      }));
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
    fetchMock.postOnce(`begin:${getUrl}`, result);
    trigger();
    await event;
  };

  const confirmDialog = () => $$('dialog', captureSkpRuns).querySelectorAll('button')[1].click();

  it('shows table entries', async () => {
    expect($('table.runssummary>tbody>tr', captureSkpRuns)).to.have.length(11);
    expect(fetchMock.lastUrl()).to.contain('exclude_dummy_page_sets=true');
    expect(fetchMock.lastUrl()).to.contain('offset=0');
    expect(fetchMock.lastUrl()).to.contain('size=10');
    expect(fetchMock.lastUrl()).to.not.contain('filter_by_logged_in_user=true');
  });

  it('filters by user', async () => {
    expect(fetchMock.lastUrl()).to.not.contain('filter_by_logged_in_user=true');
    await expectReload(() => $$('#userFilter', captureSkpRuns).click());
    expect(fetchMock.lastUrl()).to.contain('filter_by_logged_in_user=true');
  });

  it('filters by tests', async () => {
    expect(fetchMock.lastUrl()).to.contain('exclude_dummy_page_sets=true');
    await expectReload(() => $$('#testFilter', captureSkpRuns).click());
    expect(fetchMock.lastUrl()).to.not.contain('exclude_dummy_page_sets=true');
  });

  it('navigates with pages', async () => {
    expect(fetchMock.lastUrl()).to.contain('offset=0');
    const result = tasksResult1;
    result.pagination.offset = 10;
    // 'Next page' button.
    await expectReload(
      () => $('pagination-sk button.action', captureSkpRuns)[2].click(), result);
    expect(fetchMock.lastUrl()).to.contain('offset=10');
    expect($('table.runssummary>tbody>tr', captureSkpRuns)).to.have.length(5);
  });

  it('deletes tasks', async () => {
    $$('delete-icon-sk', captureSkpRuns).click();
    fetchMock.post(`begin:${deleteUrl}`, 200);
    await expectReload(confirmDialog);
    expect(fetchMock.lastOptions('begin:/_/delete').body).to.contain('"id":66');
  });

  it('reschedules tasks', async () => {
    $$('redo-icon-sk', captureSkpRuns).click();
    fetchMock.post(`begin:${redoUrl}`, 200);
    await expectReload(confirmDialog);
    expect(fetchMock.lastOptions('begin:/_/redo').body).to.contain('"id":66');
  });
});
