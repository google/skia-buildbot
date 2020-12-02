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
      expect(controlButtons()[0].hasAttribute('disabled')).to.be.true;
      expect(controlButtons()[1].hasAttribute('disabled')).to.be.true;
    } else {
      expect(controlButtons()[0].hasAttribute('disabled')).to.be.false;
      expect(controlButtons()[1].hasAttribute('disabled')).to.be.false;
    }
  };
  // Should the Next and Last buttons be disabled (are we on last page).
  const expectNextLastDisabled = (disabled: boolean) => {
    if (disabled) {
      expect(controlButtons()[2].hasAttribute('disabled')).to.be.true;
      expect(controlButtons()[3].hasAttribute('disabled')).to.be.true;
    } else {
      expect(controlButtons()[2].hasAttribute('disabled')).to.be.false;
      expect(controlButtons()[3].hasAttribute('disabled')).to.be.false;
    }
  };

  const assertButtons = (buttons: NodeList, expectations: string[]) => {
    for (let i = 0; i < buttons.length; i++) {
      expect(buttons[i].textContent).to.have.string(expectations[i]);
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
    (pageButtons()[i] as HTMLElement).click();
  };

  it('loads with control buttons', async () => {
    paginator = newInstance({ size: 10, offset: 0, total: 0 });
    // Default with no data is the 4 control(action) buttons, disabled.
    expect(pageButtons()).to.have.length(0);
    expect($('button.action:disabled', paginator)).to.have.length(4);
  });

  it('loads with page buttons', async () => {
    paginator = newInstance({ size: 10, offset: 0, total: 100 });
    // Default with enough data shows up to 5 page buttons, plus 4 controls.
    expect(pageButtons()).to.have.length(5);
    assertButtons(pageButtons(), ['1', '2', '3', '4', '5']);
    expect(currentPageButton()?.textContent).to.equal('1');
    assertButtons(clickablePageButtons(), ['2', '3', '4', '5']);
    // We begin at the first page, 'first', 'previous' buttons are disabled.
    expectFirstPreviousDisabled(true);
    expectNextLastDisabled(false);
  });

  it('allows navigation with first/last buttons', async () => {
    paginator = newInstance({ size: 10, offset: 0, total: 100 });
    assertButtons(pageButtons(), ['1', '2', '3', '4', '5']);
    let pageChangedEvent = eventPromise('page-changed');
    clickLast();
    expect(await pageChangedEvent).to.have.nested.property('detail.offset', 90);
    assertButtons(pageButtons(), ['6', '7', '8', '9', '10']);
    expect(currentPageButton()?.textContent).to.equal('10');
    assertButtons(clickablePageButtons(), ['6', '7', '8', '9']);
    expectFirstPreviousDisabled(false);
    expectNextLastDisabled(true);
    // Now return to first.
    pageChangedEvent = eventPromise('page-changed');
    clickFirst();
    expect(await pageChangedEvent).to.have.nested.property('detail.offset', 0);
    assertButtons(pageButtons(), ['1', '2', '3', '4', '5']);
    expect(currentPageButton()?.textContent).to.equal('1');
    assertButtons(clickablePageButtons(), ['2', '3', '4', '5']);
    expectFirstPreviousDisabled(true);
    expectNextLastDisabled(false);
  });

  it('allows navigation with previous/next buttons', async () => {
    paginator = newInstance({ size: 10, offset: 0, total: 100 });
    assertButtons(pageButtons(), ['1', '2', '3', '4', '5']);
    expect(currentPageButton()?.textContent).to.equal('1');
    let pageChangedEvent = eventPromise('page-changed');
    clickNext();
    expect(await pageChangedEvent).to.have.nested.property('detail.offset', 10);
    // Page buttons don't scroll until active page button is in the middle.
    assertButtons(pageButtons(), ['1', '2', '3', '4', '5']);
    expect(currentPageButton()?.textContent).to.equal('2');
    assertButtons(clickablePageButtons(), ['1', '3', '4', '5']);
    expectFirstPreviousDisabled(false);
    expectNextLastDisabled(false);
    // Button number scroll when we go two more.
    pageChangedEvent = eventPromise('page-changed');
    clickNext();
    expect(await pageChangedEvent).to.have.nested.property('detail.offset', 20);
    clickNext();
    assertButtons(pageButtons(), ['2', '3', '4', '5', '6']);
    expect(currentPageButton()?.textContent).to.equal('4');
    assertButtons(clickablePageButtons(), ['2', '3', '5', '6']);
    expectFirstPreviousDisabled(false);
    expectNextLastDisabled(false);
    // Now go back one.
    clickPrevious();
    assertButtons(pageButtons(), ['1', '2', '3', '4', '5']);
    expect(currentPageButton()?.textContent).to.equal('3');
    assertButtons(clickablePageButtons(), ['1', '2', '4', '5']);
    expectFirstPreviousDisabled(false);
    expectNextLastDisabled(false);
  });

  it('allows navigation with page buttons', async () => {
    paginator = newInstance({ size: 10, offset: 0, total: 100 });
    assertButtons(pageButtons(), ['1', '2', '3', '4', '5']);
    // Go to page 5.
    const pageChangedEvent = eventPromise('page-changed');
    clickNthPageButton(4);
    expect(await pageChangedEvent).to.have.nested.property('detail.offset', 40);
    assertButtons(pageButtons(), ['3', '4', '5', '6', '7']);
    expect(currentPageButton()?.textContent).to.equal('5');
    assertButtons(clickablePageButtons(), ['3', '4', '6', '7']);
    // Go to page 7, then 9, then 10.
    clickNthPageButton(4);
    assertButtons(pageButtons(), ['5', '6', '7', '8', '9']);
    expect(currentPageButton()?.textContent).to.equal('7');
    clickNthPageButton(4);
    assertButtons(pageButtons(), ['6', '7', '8', '9', '10']);
    expect(currentPageButton()?.textContent).to.equal('9');
    clickNthPageButton(4);
    assertButtons(pageButtons(), ['6', '7', '8', '9', '10']);
    expect(currentPageButton()?.textContent).to.equal('10');
    expectNextLastDisabled(true);
  });
});
