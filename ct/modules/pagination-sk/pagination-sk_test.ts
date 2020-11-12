import './index';

import { expect } from 'chai';
import { $ } from 'common-sk/modules/dom';
import { PaginationSk } from './pagination-sk';

import {
  ResponsePagination,
} from '../json';
import {
  eventPromise,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';

describe('pagination-sk', () => {
  const newInstance = (() => {
    const factory = setUpElementUnderTest<PaginationSk>('pagination-sk');
    return (paginationData: ResponsePagination) => factory((el: PaginationSk) => {
      if (paginationData) { el.pagination = paginationData; }
    });
  })();

  let paginator: HTMLElement;
  // Array of 4 buttons, First, Previous, Next, Last, respectively.
  const controlButtons = () => paginator.querySelectorAll('button.action');
  // All present numbered page buttons.
  const pageButtons = () => paginator.querySelectorAll('button:not(.action)');
  // Button of current page, disabled.
  const currentPageButton = () => paginator.querySelector('button:not(.action)[disabled]');
  // All page buttons other than currentPageButton.
  const clickablePageButtons = () => paginator.querySelectorAll('button:not(.action):not([disabled])');
  // Should the First and Previous buttons be disabled (are we on page zero).
  const expectFirstPreviousDisabled = (disabled: boolean) => {
    if (disabled) {
      expect(controlButtons()[0]).to.match(new RegExp('[disabled]'));
      expect(controlButtons()[1]).to.match(new RegExp('[disabled]'));
    } else {
      expect(controlButtons()[0]).to.not.match(new RegExp('[disabled]'));
      expect(controlButtons()[1]).to.not.match(new RegExp('[disabled]'));
    }
  };
  // Should the Next and Last buttons be disabled (are we on last page).
  const expectNextLastDisabled = (disabled: boolean) => {
    if (disabled) {
      expect(controlButtons()[2]).to.match(new RegExp('[disabled]'));
      expect(controlButtons()[3]).to.match(new RegExp('[disabled]'));
    } else {
      expect(controlButtons()[2]).to.not.match(new RegExp('[disabled]'));
      expect(controlButtons()[3]).to.not.match(new RegExp('[disabled]'));
    }
  };

  // Helpers for clicking control and page buttons.
  const clickFirst = () => {
    (controlButtons()[0] as HTMLElement).click();
  };

  const clickLast = () => {
    (controlButtons()[3] as HTMLElement).click();
  };

  const clickPrevious = () => {
    (controlButtons()[1] as HTMLElement).click();
  };

  const clickNext = () => {
    (controlButtons()[2] as HTMLElement).click();
  };

  // Click the page button at the provided 0-based index. e.g. for page
  // buttons ['5','6','7'] (where showPages===3),
  // clickNthPageButton(2) will click the 'page 7' button.
  const clickNthPageButton = (i: number) => {
    (controlButtons()[i] as HTMLElement).click();
  };

  it('loads with control buttons', async () => {
    paginator = newInstance({ size: 0, offset: 0, total: 0 });
    // Default with no data is the 4 control(action) buttons, disabled.
    expect(pageButtons()).to.have.length(0);
    expect($('button.action:disabled', paginator)).to.have.length(4);
  });

  it('loads with page buttons', async () => {
    paginator = newInstance({ size: 10, offset: 0, total: 100 });
    // Default with enough data shows up to 5 page buttons, plus 4 controls.
    expect(pageButtons()).to.have.length(5);
    expect(pageButtons()).to.have.members(['1', '2', '3', '4', '5']);
    expect(currentPageButton()).to.have('1');
    expect(clickablePageButtons()).to.have.members(['2', '3', '4', '5']);
    // We begin at the first page, 'first', 'previous' buttons are disabled.
    expectFirstPreviousDisabled(true);
    expectNextLastDisabled(false);
  });

  it('allows navigation with first/last buttons', async () => {
    paginator = newInstance({ size: 10, offset: 0, total: 100 });
    expect(pageButtons()).to.have.members(['1', '2', '3', '4', '5']);
    let pageChangedEvent = eventPromise('page-changed');
    clickLast();
    expect(await pageChangedEvent).to.have.nested.property('detail.offset', 90);
    expect(pageButtons()).to.have.members(['6', '7', '8', '9', '10']);
    expect(currentPageButton()).to.have('10');
    expect(clickablePageButtons()).to.have.members(['6', '7', '8', '9']);
    expectFirstPreviousDisabled(false);
    expectNextLastDisabled(true);
    // Now return to first.
    pageChangedEvent = eventPromise('page-changed');
    clickFirst();
    expect(await pageChangedEvent).to.have.nested.property('detail.offset', 0);
    expect(pageButtons()).to.have.members(['1', '2', '3', '4', '5']);
    expect(currentPageButton()).to.have('1');
    expect(clickablePageButtons()).to.have.members(['2', '3', '4', '5']);
    expectFirstPreviousDisabled(true);
    expectNextLastDisabled(false);
  });

  it('allows navigation with previous/next buttons', async () => {
    paginator = newInstance({ size: 10, offset: 0, total: 100 });
    expect(pageButtons()).to.have.members(['1', '2', '3', '4', '5']);
    expect(currentPageButton()).to.have('1');
    let pageChangedEvent = eventPromise('page-changed');
    clickNext();
    expect(await pageChangedEvent).to.have.nested.property('detail.offset', 10);
    // Page buttons don't scroll until active page button is in the middle.
    expect(pageButtons()).to.have.members(['1', '2', '3', '4', '5']);
    expect(currentPageButton()).to.have('2');
    expect(clickablePageButtons()).to.have.members(['1', '3', '4', '5']);
    expectFirstPreviousDisabled(false);
    expectNextLastDisabled(false);
    // Button number scroll when we go two more.
    pageChangedEvent = eventPromise('page-changed');
    clickNext();
    expect(await pageChangedEvent).to.have.nested.property('detail.offset', 20);
    clickNext();
    expect(pageButtons()).to.have.members(['2', '3', '4', '5', '6']);
    expect(currentPageButton()).to.have('4');
    expect(clickablePageButtons()).to.have.members(['2', '3', '5', '6']);
    expectFirstPreviousDisabled(false);
    expectNextLastDisabled(false);
    // Now go back one.
    clickPrevious();
    expect(pageButtons()).to.have.members(['1', '2', '3', '4', '5']);
    expect(currentPageButton()).to.have('3');
    expect(clickablePageButtons()).to.have.members(['1', '2', '4', '5']);
    expectFirstPreviousDisabled(false);
    expectNextLastDisabled(false);
  });

  it('allows navigation with page buttons', async () => {
    paginator = newInstance({ size: 10, offset: 0, total: 100 });
    expect(pageButtons()).to.have.members(['1', '2', '3', '4', '5']);
    // Go to page 5.
    const pageChangedEvent = eventPromise('page-changed');
    clickNthPageButton(4);
    expect(await pageChangedEvent).to.have.nested.property('detail.offset', 40);
    expect(pageButtons()).to.have.members(['3', '4', '5', '6', '7']);
    expect(currentPageButton()).to.have('5');
    expect(clickablePageButtons()).to.have.members(['3', '4', '6', '7']);
    // Go to page 7, then 9, then 10.
    clickNthPageButton(4);
    expect(pageButtons()).to.have.members(['5', '6', '7', '8', '9']);
    expect(currentPageButton()).to.have('7');
    clickNthPageButton(4);
    expect(pageButtons()).to.have.members(['6', '7', '8', '9', '10']);
    expect(currentPageButton()).to.have('9');
    clickNthPageButton(4);
    expect(pageButtons()).to.have.members(['6', '7', '8', '9', '10']);
    expect(currentPageButton()).to.have('10');
    expectNextLastDisabled(true);
  });
});
