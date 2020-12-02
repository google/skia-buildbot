import './index';

import { $, $$ } from 'common-sk/modules/dom';

import {
  eventPromise,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';

describe('pagination-sk', () => {
  const newInstance = (() => {
    const factory = setUpElementUnderTest('pagination-sk');
    return (paginationData) => factory((el) => {
      if (paginationData) { el.pagination = paginationData; }
    });
  })();

  let paginator;
  // Array of 4 buttons, First, Previous, Next, Last, respectively.
  const controlButtons = () => paginator.querySelectorAll('button.action');
  // All present numbered page buttons.
  const pageButtons = () => paginator.querySelectorAll('button:not(.action)');
  // Button of current page, disabled.
  const currentPageButton = () => paginator.querySelector('button:not(.action)[disabled]');
  // All page buttons other than currentPageButton.
  const clickablePageButtons = () => paginator.querySelectorAll('button:not(.action):not([disabled])');
  // Should the First and Previous buttons be disabled (are we on page zero).
  const expectFirstPreviousDisabled = (disabled) => {
    if (disabled) {
      expect(controlButtons()[0]).to.match('[disabled]');
      expect(controlButtons()[1]).to.match('[disabled]');
    } else {
      expect(controlButtons()[0]).to.not.match('[disabled]');
      expect(controlButtons()[1]).to.not.match('[disabled]');
    }
  };
  // Should the Next and Last buttons be disabled (are we on last page).
  const expectNextLastDisabled = (disabled) => {
    if (disabled) {
      expect(controlButtons()[2]).to.match('[disabled]');
      expect(controlButtons()[3]).to.match('[disabled]');
    } else {
      expect(controlButtons()[2]).to.not.match('[disabled]');
      expect(controlButtons()[3]).to.not.match('[disabled]');
    }
  };

  // Helpers for clicking control and page buttons.
  const clickFirst = () => {
    controlButtons()[0].click();
  };

  const clickLast = () => {
    controlButtons()[3].click();
  };

  const clickPrevious = () => {
    controlButtons()[1].click();
  };

  const clickNext = () => {
    controlButtons()[2].click();
  };

  // Click the page button at the provided 0-based index. e.g. for page
  // buttons ['5','6','7'] (where showPages===3),
  // clickNthPageButton(2) will click the 'page 7' button.
  const clickNthPageButton = (i) => {
    pageButtons()[i].click();
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
    expect(pageButtons()).to.have.text(['1', '2', '3', '4', '5']);
    expect(currentPageButton()).to.have.text('1');
    expect(clickablePageButtons()).to.have.text(['2', '3', '4', '5']);
    // We begin at the first page, 'first', 'previous' buttons are disabled.
    expectFirstPreviousDisabled(true);
    expectNextLastDisabled(false);
  });

  it('allows navigation with first/last buttons', async () => {
    paginator = newInstance({ size: 10, offset: 0, total: 100 });
    expect(pageButtons()).to.have.text(['1', '2', '3', '4', '5']);
    let pageChangedEvent = eventPromise('page-changed');
    clickLast();
    expect(await pageChangedEvent).to.have.nested.property('detail.offset', 90);
    expect(pageButtons()).to.have.text(['6', '7', '8', '9', '10']);
    expect(currentPageButton()).to.have.text('10');
    expect(clickablePageButtons()).to.have.text(['6', '7', '8', '9']);
    expectFirstPreviousDisabled(false);
    expectNextLastDisabled(true);
    // Now return to first.
    pageChangedEvent = eventPromise('page-changed');
    clickFirst();
    expect(await pageChangedEvent).to.have.nested.property('detail.offset', 0);
    expect(pageButtons()).to.have.text(['1', '2', '3', '4', '5']);
    expect(currentPageButton()).to.have.text('1');
    expect(clickablePageButtons()).to.have.text(['2', '3', '4', '5']);
    expectFirstPreviousDisabled(true);
    expectNextLastDisabled(false);
  });

  it('allows navigation with previous/next buttons', async () => {
    paginator = newInstance({ size: 10, offset: 0, total: 100 });
    expect(pageButtons()).to.have.text(['1', '2', '3', '4', '5']);
    expect(currentPageButton()).to.have.text('1');
    let pageChangedEvent = eventPromise('page-changed');
    clickNext();
    expect(await pageChangedEvent).to.have.nested.property('detail.offset', 10);
    // Page buttons don't scroll until active page button is in the middle.
    expect(pageButtons()).to.have.text(['1', '2', '3', '4', '5']);
    expect(currentPageButton()).to.have.text('2');
    expect(clickablePageButtons()).to.have.text(['1', '3', '4', '5']);
    expectFirstPreviousDisabled(false);
    expectNextLastDisabled(false);
    // Button number scroll when we go two more.
    pageChangedEvent = eventPromise('page-changed');
    clickNext();
    expect(await pageChangedEvent).to.have.nested.property('detail.offset', 20);
    clickNext();
    expect(pageButtons()).to.have.text(['2', '3', '4', '5', '6']);
    expect(currentPageButton()).to.have.text('4');
    expect(clickablePageButtons()).to.have.text(['2', '3', '5', '6']);
    expectFirstPreviousDisabled(false);
    expectNextLastDisabled(false);
    // Now go back one.
    clickPrevious();
    expect(pageButtons()).to.have.text(['1', '2', '3', '4', '5']);
    expect(currentPageButton()).to.have.text('3');
    expect(clickablePageButtons()).to.have.text(['1', '2', '4', '5']);
    expectFirstPreviousDisabled(false);
    expectNextLastDisabled(false);
  });

  it('allows navigation with page buttons', async () => {
    paginator = newInstance({ size: 10, offset: 0, total: 100 });
    expect(pageButtons()).to.have.text(['1', '2', '3', '4', '5']);
    // Go to page 5.
    const pageChangedEvent = eventPromise('page-changed');
    clickNthPageButton(4);
    expect(await pageChangedEvent).to.have.nested.property('detail.offset', 40);
    expect(pageButtons()).to.have.text(['3', '4', '5', '6', '7']);
    expect(currentPageButton()).to.have.text('5');
    expect(clickablePageButtons()).to.have.text(['3', '4', '6', '7']);
    // Go to page 7, then 9, then 10.
    clickNthPageButton(4);
    expect(pageButtons()).to.have.text(['5', '6', '7', '8', '9']);
    expect(currentPageButton()).to.have.text('7');
    clickNthPageButton(4);
    expect(pageButtons()).to.have.text(['6', '7', '8', '9', '10']);
    expect(currentPageButton()).to.have.text('9');
    clickNthPageButton(4);
    expect(pageButtons()).to.have.text(['6', '7', '8', '9', '10']);
    expect(currentPageButton()).to.have.text('10');
    expectNextLastDisabled(true);
  });
});
