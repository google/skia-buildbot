import './index';

import { expect } from 'chai';
import { eventPromise, noEventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { PaginationSk, PaginationSkPageChangedEventDetail } from './pagination-sk';
import { PaginationSkPO } from './pagination-sk_po';

describe('pagination-sk', () => {
  const newInstance = setUpElementUnderTest<PaginationSk>('pagination-sk');

  let paginationSk: PaginationSk;
  let paginationSkPO: PaginationSkPO;

  beforeEach(() => {
    paginationSk = newInstance((el) => {
      el.setAttribute('offset', '0');
      el.setAttribute('total', '127');
      el.setAttribute('page_size', '20');
    });
    paginationSkPO = new PaginationSkPO(paginationSk);
  });

  it('reflects attributes as properties', () => {
    expect(paginationSk.offset).to.equal(0);
    expect(paginationSk.total).to.equal(127);
    expect(paginationSk.page_size).to.equal(20);
  });

  describe('html layout', () => {
    it('enables and disables buttons based on the current page', async () => {
      expect(await paginationSkPO.isPrevBtnDisabled()).to.be.true;
      expect(await paginationSkPO.isNextBtnDisabled()).to.be.false;
      expect(await paginationSkPO.isSkipBtnDisabled()).to.be.false;

      paginationSk.offset = 20;
      expect(await paginationSkPO.isPrevBtnDisabled()).to.be.false;
      expect(await paginationSkPO.isNextBtnDisabled()).to.be.false;
      expect(await paginationSkPO.isSkipBtnDisabled()).to.be.false;

      paginationSk.offset = 40;
      expect(await paginationSkPO.isPrevBtnDisabled()).to.be.false;
      expect(await paginationSkPO.isNextBtnDisabled()).to.be.false;
      expect(await paginationSkPO.isSkipBtnDisabled()).to.be.true;

      paginationSk.offset = 120;
      expect(await paginationSkPO.isPrevBtnDisabled()).to.be.false;
      expect(await paginationSkPO.isNextBtnDisabled()).to.be.true;
      expect(await paginationSkPO.isSkipBtnDisabled()).to.be.true;
    });

    it('displays the current page', async () => {
      expect(await paginationSkPO.getCurrentPage()).to.equal(1);
      paginationSk.offset = 20;
      expect(await paginationSkPO.getCurrentPage()).to.equal(2);
      paginationSk.offset = 40;
      expect(await paginationSkPO.getCurrentPage()).to.equal(3);
    });
  }); // end describe('html layout')

  describe('paging behavior', () => {
    it('does not auto update the page offset', async () => {
      expect(await paginationSkPO.getCurrentPage()).to.equal(1);
      await paginationSkPO.clickNextBtn();
      expect(await paginationSkPO.getCurrentPage()).to.equal(1);
      await paginationSkPO.clickSkipBtn();
      expect(await paginationSkPO.getCurrentPage()).to.equal(1);
      await paginationSkPO.clickPrevBtn();
      expect(await paginationSkPO.getCurrentPage()).to.equal(1);
    });

    it('creates page events', async () => {
      // We start at page 1, so the "prev" button is disabled, and clicking it has no effect.
      const noPaginationEvent = noEventPromise('page-changed');
      await paginationSkPO.clickPrevBtn();
      await noPaginationEvent;

      // Still at page 1 because nothing happened.
      let paginationEvent = eventPromise<CustomEvent<PaginationSkPageChangedEventDetail>>('page-changed');
      await paginationSkPO.clickNextBtn();
      expect((await paginationEvent).detail.delta).to.equal(1);

      // Still at page 1 because the component does not auto update the offset.
      paginationEvent = eventPromise<CustomEvent<PaginationSkPageChangedEventDetail>>('page-changed');
      await paginationSkPO.clickSkipBtn();
      expect((await paginationEvent).detail.delta).to.equal(5);

      // Move the offset by one page so as to enable the "prev" button.
      paginationSk.offset = 20;
      paginationEvent = eventPromise<CustomEvent<PaginationSkPageChangedEventDetail>>('page-changed');
      await paginationSkPO.clickPrevBtn();
      expect((await paginationEvent).detail.delta).to.equal(-1);
    });
  }); // end describe('paging behavior')
});
