import './index';

import sinon from 'sinon';
import { expect } from 'chai';
import { $, $$ } from 'common-sk/modules/dom';
import fetchMock from 'fetch-mock';
import { TaskQueueSk } from './task-queue-sk';

import {
  singleResultCanDelete, singleResultNoDelete, resultSetOneItem, resultSetTwoItems,
} from './test_data';
import {
  eventPromise,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';

describe('task-queue-sk', () => {
  const newInstance = setUpElementUnderTest('task-queue-sk');
  fetchMock.config.overwriteRoutes = false;

  const loadTable = async () => {
    const event = eventPromise('end-task');
    const taskTableSk = newInstance();
    await event;
    return taskTableSk;
  };
  const loadTableWithReplies = async (replies: any[]) => {
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
    sinon.restore();
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
    fetchMock.postOnce((url, options) => url.startsWith('/_/delete_') && options.body === JSON.stringify({ id: 1 }), 200);
    sinon.stub(window, 'confirm').returns(true);
    sinon.stub(window, 'alert').returns(true);
    ($$('delete-icon-sk', table) as HTMLElement).click();
    expect($$('dialog', table)).to.have.property('open', true);
  });

  it('task details works', async () => {
    const table = await loadTableWithReplies([resultSetOneItem]);

    expect($$('.dialog-background', table)).to.have.class('hidden');
    ($$('.details', table) as HTMLElement).click();

    expect($$('.dialog-background', table)).to.not.have.class('hidden');
  });
});
