import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';

import {
  singleResultCanDelete, singleResultNoDelete, resultSetOneItem, resultSetTwoItems,
} from './test_data';
import {
  eventPromise,
  setUpElementUnderTest,
} from '../../../golden/modules/test_util';

describe('task-queue-sk', () => {
  const newInstance = setUpElementUnderTest('task-queue-sk');
  fetchMock.config.overwriteRoutes = false;

  const loadTable = async () => {
    const event = eventPromise('end-task');
    const taskTableSk = newInstance();
    await event;
    return taskTableSk;
  };
  const loadTableWithReplies = async (replies) => {
    const kNumTaskQueries = 16;
    const replyCount = replies.length;
    expect(replyCount).to.be.most(kNumTaskQueries);
    for (let i = 0; i < replyCount; ++i) {
      fetchMock.postOnce('begin:/_/get_', replies[i]);
    }
    fetchMock.post('begin:/_/get_', 200, { repeat: kNumTaskQueries - replyCount });

    return loadTable();
  };

  afterEach(() => {
    //  Check all mock fetches called at least once and reset.
    expect(fetchMock.done()).to.be.true;
    fetchMock.reset();
  });

  it('is a valid testcase', async () => {
    // await loadTable();
    expect(1).to.equal(1);
  });

  it('shows table entries', async () => {
    // Return some results for 2 of the 16 task queries.
    fetchMock.postOnce('begin:/_/get_', resultSetOneItem);
    fetchMock.postOnce('begin:/_/get_', resultSetTwoItems);
    fetchMock.post('begin:/_/get_', 200, { repeat: 14 });
    /* fetchMock.post('/_/get_chromium_analysis_tasks?not_completed=true&size=100', []);
    fetchMock.post('/_/get_metrics_analysis_tasks?not_completed=true&size=100', []);
    fetchMock.post('/_/get_capture_skp_tasks?not_completed=true&size=100', []);
    fetchMock.post('/_/get_lua_script_tasks?not_completed=true&size=100', []);
    fetchMock.post('/_/get_chromium_build_tasks?not_completed=true&size=100', []);
    fetchMock.post('/_/get_recreate_page_sets_tasks?not_completed=true&size=100', []);
    fetchMock.post('/_/get_recreate_webpage_archives_tasks?not_completed=true&size=100', []);
    */


    const table = await loadTable();
    // (3 items + 1 header row) * 6 columns
    expect($('td', table).length).to.equal(24);
  });
  it('delete option shown', async () => {
    fetchMock.postOnce('begin:/_/get_', singleResultCanDelete);
    fetchMock.post('begin:/_/get_', 200, { repeat: 15 });
    const table = await loadTable();
    expect($$('delete-icon-sk')).to.have.property('hidden', false);
  });
  it('delete option hidden', async () => {
    fetchMock.postOnce('begin:/_/get_', singleResultNoDelete);
    fetchMock.post('begin:/_/get_', 200, { repeat: 15 });
    const table = await loadTable();
    expect($$('delete-icon-sk')).to.have.property('hidden', true);
  });

  it('delete flow works', async () => {
    fetchMock.postOnce('begin:/_/get_', singleResultCanDelete);
    fetchMock.post('begin:/_/get_', 200, { repeat: 15 });
    const table = await loadTable();
    expect($$('dialog')).to.have.property('open', false);
    $$('delete-icon-sk').click();
    expect($$('dialog')).to.have.property('open', true);
    fetchMock.postOnce((url, options) => url.startsWith('/_/delete_') && options.body === JSON.stringify({ id: 1 }), 200);
    $$('dialog').querySelectorAll('button')[1].click();
    expect($$('dialog')).to.have.property('open', false);
  });

  it('task details works', async () => {
    const table = await loadTableWithReplies([resultSetOneItem]);
    expect($$('.dialog-background')).to.have.nested.property('style.display', '');
    $('a').pop().click();
    expect($$('.dialog-background')).to.have.nested.property('style.display', 'block');
  });
});
