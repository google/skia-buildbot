import './index';

import { $, $$ } from 'common-sk/modules/dom';
import fetchMock from 'fetch-mock';
import { expect } from 'chai';
import {
  changelistSummaries_5,
  changelistSummaries_5_offset5,
  changelistSummaries_5_offset10,
  empty,
} from './test_data';
import {
  eventPromise,
  expectQueryStringToEqual,
  setQueryString,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';
import { ChangelistsPageSk } from './changelists-page-sk';
import { PaginationSk } from '../pagination-sk/pagination-sk';
import { PaginationSkPO } from '../pagination-sk/pagination-sk_po';

describe('changelists-page-sk', () => {
  const newInstance = setUpElementUnderTest<ChangelistsPageSk>('changelists-page-sk');

  // Instantiate page; wait for RPCs to complete and for the page to render.
  const loadChangelistsPageSk = async () => {
    const endTask = eventPromise('end-task');
    const instance = newInstance();
    await endTask;
    return instance;
  };

  beforeEach(async () => {
    // Clear out any query params we might have to not mess with our current state.
    setQueryString('');
  });

  afterEach(() => {
    expect(fetchMock.done()).to.be.true; // All mock RPCs called at least once.
    // Completely remove the mocking which allows each test
    // to be able to mess with the mocked routes w/o impacting other tests.
    fetchMock.reset();
  });

  describe('html layout', () => {
    let changelistsPageSk: ChangelistsPageSk;

    beforeEach(async () => {
      // These are the default offset/page_size params
      fetchMock.get('/json/v2/changelists?offset=0&size=50&active=true', changelistSummaries_5);

      changelistsPageSk = await loadChangelistsPageSk();
    });

    it('should make a table with 5 rows in the body', () => {
      const tbl = $$('table', changelistsPageSk);
      expect(tbl).to.not.be.null;
      const rows = $('tbody tr', changelistsPageSk);
      expect(rows.length).to.equal(5); // one row per item in changelistSummaries_5
    });

    it('has icons that indicate the status', () => {
      const rows = $('tbody tr', changelistsPageSk);
      // First row has an open CL.
      let icon = $$('cached-icon-sk', rows[0]);
      expect(icon).to.not.be.null;
      // Fourth row has an abandoned CL.
      icon = $$('block-icon-sk', rows[3]);
      expect(icon).to.not.be.null;
      // Fifth row has an closed CL.
      icon = $$('done-icon-sk', rows[4]);
      expect(icon).to.not.be.null;
    });
  }); // end describe('html layout')

  describe('api calls', () => {
    it('includes pagination params in request to changelists', async () => {
      setQueryString('?offset=100&page_size=10');
      fetchMock.get('/json/v2/changelists?offset=100&size=10&active=true', empty);
      await loadChangelistsPageSk();
    });

    it('includes the active params unless show_all is set', async () => {
      setQueryString('?offset=100&page_size=10&show_all=true');
      fetchMock.get('/json/v2/changelists?offset=100&size=10', empty);
      await loadChangelistsPageSk();
    });
  }); // end describe('api calls')

  describe('navigation', () => {
    it('responds to the browser back/forward buttons', async () => {
      // These are the default offset/page_size params. This request will be made when we test the
      // ?hello=world query string.
      fetchMock.get('/json/v2/changelists?offset=0&size=50&active=true', changelistSummaries_5);

      // First page of results.
      fetchMock.get(
        '/json/v2/changelists?offset=0&size=5&active=true',
        changelistSummaries_5,
      );

      // Second page of results.
      fetchMock.get(
        '/json/v2/changelists?offset=5&size=5&active=true',
        changelistSummaries_5_offset5,
      );

      // Third page of results.
      fetchMock.get(
        '/json/v2/changelists?offset=10&size=5&active=true',
        changelistSummaries_5_offset10,
      );

      // Random query string value before instantiating the component under
      // test. We'll test that we can navigate back to this URL using the
      // browser's back button.
      setQueryString('?hello=world');

      // Query string at component instantiation. This specifies the page size
      // required for the mock RPCs above to work.
      setQueryString('?page_size=5');

      // Instantiate component.
      const changelistsPageSk = await loadChangelistsPageSk();
      expectQueryStringToEqual('?page_size=5');
      expectFirstPage(changelistsPageSk);

      await goToNextPage(changelistsPageSk);
      expectQueryStringToEqual('?offset=5&page_size=5');
      expectSecondPage(changelistsPageSk);

      await goToNextPage(changelistsPageSk);
      expectQueryStringToEqual('?offset=10&page_size=5');
      expectThirdPage(changelistsPageSk);

      await goBack();
      expectQueryStringToEqual('?offset=5&page_size=5');
      expectSecondPage(changelistsPageSk);

      // State at component instantiation.
      await goBack();
      expectQueryStringToEqual('?page_size=5');
      expectFirstPage(changelistsPageSk);

      // State before the component was instantiated.
      await goBack();
      expectQueryStringToEqual('?hello=world');

      await goForward();
      expectQueryStringToEqual('?page_size=5');
      expectFirstPage(changelistsPageSk);

      await goForward();
      expectQueryStringToEqual('?offset=5&page_size=5');
      expectSecondPage(changelistsPageSk);

      await goForward();
      expectQueryStringToEqual('?offset=10&page_size=5');
      expectThirdPage(changelistsPageSk);
    });
  }); // end describe('navigation')

  describe('dynamic content', () => {
    it('responds to clicking the show all checkbox', async () => {
      fetchMock.get('/json/v2/changelists?offset=0&size=5&active=true', empty);
      fetchMock.get('/json/v2/changelists?offset=0&size=5', empty);

      setQueryString('?page_size=5');
      const changelistsPageSk = await loadChangelistsPageSk();

      // click on the input inside the checkbox, otherwise, we see double
      // events, since checkbox-sk "re-throws" the click event.
      const showAllBox = $$<HTMLInputElement>('.controls checkbox-sk input', changelistsPageSk)!;
      expect(showAllBox).to.not.be.null;
      expectQueryStringToEqual('?page_size=5');
      showAllBox.click();
      expectQueryStringToEqual('?page_size=5&show_all=true');
      showAllBox.click();
      expectQueryStringToEqual('?page_size=5');
      showAllBox.click();
      expectQueryStringToEqual('?page_size=5&show_all=true');
    });
  }); // end describe('dynamic content')
});

function goToNextPage(changelistsPageSk: ChangelistsPageSk): Promise<Event> {
  const event = eventPromise('end-task');
  new PaginationSkPO($$<PaginationSk>('pagination-sk', changelistsPageSk)!).clickNextBtn();
  return event;
}

function goBack() {
  const event = eventPromise('end-task');
  history.back();
  return event;
}

function goForward() {
  const event = eventPromise('end-task');
  history.forward();
  return event;
}

function expectFirstPage(changelistsPageSk: ChangelistsPageSk) {
  expect($<HTMLTableDataCellElement>('td.owner', changelistsPageSk)[0].innerText)
    .to.contain('alpha');
}

function expectSecondPage(changelistsPageSk: ChangelistsPageSk) {
  expect($<HTMLTableDataCellElement>('td.owner', changelistsPageSk)[0].innerText)
    .to.contain('zeta');
}

function expectThirdPage(changelistsPageSk: ChangelistsPageSk) {
  expect($<HTMLTableDataCellElement>('td.owner', changelistsPageSk)[0].innerText)
    .to.contain('lambda');
}
