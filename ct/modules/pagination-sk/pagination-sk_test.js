import './index';

import { $, $$ } from 'common-sk/modules/dom';

import {
  eventPromise,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';

describe('pagination-sk', () => {
  const newInstance = (() => {
    const factory = setUpElementUnderTest('pagination-sk');
    return (paginationData) => factory((el) => { if (paginationData) { el.pagination = paginationData; } });
  })();

  let paginator;
  const controlButtons = () => paginator.querySelectorAll('button.action');
  const pageButtons = () => paginator.querySelectorAll('button:not(.action)');
  const activePageButton = () => paginator.querySelector('button:not(.action).disabled');
  const expectFirstPreviousDisabled = () => {

  };

  it('loads with control buttons', async () => {
    paginator = newInstance();
    // Default with no data is the 4 control(action) buttons, disabled.
    expect(pageButtons()).to.have.length(0);
    expect($('button.action:disabled', paginator)).to.have.length(4);
  });

  it('loads with page buttons', async () => {
    paginator = newInstance({ size: 10, offset: 0, total: 100 });
    // Default with enough data shows up to 5 page buttons, plus 4 controls.
    expect(pageButtons()).to.have.length(5);
    console.log(paginator.querySelectorAll('button:not(.action)'));
    expect(paginator.querySelectorAll('button:not(.action)')).to.contain.text(['1', '2', '3', '4', '5']);
   // expect(activePageButton()).to.have.text('1');
    // We begin at the first page, 'first', 'previous' buttons are disabled.
    expect(controlButtons()[0]).to.match('[disabled]');
  });
/*
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

    expect($$('.dialog-background', table)).to.have.class('hidden');
    $$('.details', table).click();

    expect($$('.dialog-background', table)).to.not.have.class('hidden');
  }); */
});
