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

  it('shows table entries', async () => {
    // Return some results for 2 of the 16 task queries.
    const table = await loadTableWithReplies([resultSetOneItem, resultSetTwoItems]);

    // (3 items) * 6 columns
    expect($('td', table).length).to.equal(18);
  });

  it('delete option shown', async () => {
    const table = await loadTableWithReplies([singleResultCanDelete]);

    expect($$('delete-icon-sk', table)).to.have.property('hidden', false);
  });

  it('delete option hidden', async () => {
    const table = await loadTableWithReplies([singleResultNoDelete]);

    expect($$('delete-icon-sk', table)).to.have.property('hidden', true);
  });

  it('delete flow works', async () => {
    const table = await loadTableWithReplies([singleResultCanDelete]);

    expect($$('dialog', table)).to.have.property('open', false);
    $$('delete-icon-sk', table).click();
    expect($$('dialog', table)).to.have.property('open', true);
    fetchMock.postOnce((url, options) => url.startsWith('/_/delete_') && options.body === JSON.stringify({ id: 1 }), 200);
    // TODO(weston): Update common-sk/confirm-dialog-sk to make this less
    // brittle.
    $$('dialog', table).querySelectorAll('button')[1].click();
    expect($$('dialog', table)).to.have.property('open', false);
  });

  it('task details works', async () => {
    const table = await loadTableWithReplies([resultSetOneItem]);

    expect($$('.dialog-background', table).className).to.contain('hidden');
    $$('.details', table).click();

    expect($$('.dialog-background', table).className).to.not.contain('hidden');
  });
});
