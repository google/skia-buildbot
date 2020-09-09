import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
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

describe('changelists-page-sk', () => {
  const newInstance = setUpElementUnderTest('changelists-page-sk');

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

    // These are the default offset/page_size params
    fetchMock.get('/json/v1/changelists?offset=0&size=50&active=true', changelistSummaries_5);
  });

  afterEach(() => {
    // Completely remove the mocking which allows each test
    // to be able to mess with the mocked routes w/o impacting other tests.
    fetchMock.reset();
  });

  describe('html layout', () => {
    let changelistsPageSk;
    beforeEach(async () => {
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
    let changelistsPageSk;
    beforeEach(async () => {
      changelistsPageSk = await loadChangelistsPageSk();
    });

    it('includes pagination params in request to changelists', async () => {
      fetchMock.resetHistory();

      fetchMock.get('/json/v1/changelists?offset=100&size=10', empty);
      // pretend these were loaded in via stateReflector
      changelistsPageSk._offset = 100;
      changelistsPageSk._page_size = 10;
      changelistsPageSk._showAll = true;

      await changelistsPageSk._fetch();
    });

    it('includes the active params unless show_all is set', async () => {
      fetchMock.resetHistory();

      fetchMock.get('/json/v1/changelists?offset=100&size=10&active=true', empty);
      // pretend these were loaded in via stateReflector
      changelistsPageSk._offset = 100;
      changelistsPageSk._page_size = 10;
      changelistsPageSk._showAll = false;

      await changelistsPageSk._fetch();
    });
  }); // end describe('api calls')

  describe('navigation', () => {
    it('responds to the browser back/forward buttons', async () => {
      // First page of results.
      fetchMock.get(
        '/json/v1/changelists?offset=0&size=5&active=true',
        changelistSummaries_5,
      );
      // Second page of results.
      fetchMock.get(
        '/json/v1/changelists?offset=5&size=5&active=true',
        changelistSummaries_5_offset5,
      );
      // Third page of results.
      fetchMock.get(
        '/json/v1/changelists?offset=10&size=5&active=true',
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
      expectFirstPage();

      await goToNextPage(changelistsPageSk);
      expectQueryStringToEqual('?offset=5&page_size=5');
      expectSecondPage();

      await goToNextPage(changelistsPageSk);
      expectQueryStringToEqual('?offset=10&page_size=5');
      expectThirdPage();

      await goBack();
      expectQueryStringToEqual('?offset=5&page_size=5');
      expectSecondPage();

      // State at component instantiation.
      await goBack();
      expectQueryStringToEqual('?page_size=5');
      expectFirstPage();

      // State before the component was instantiated.
      await goBack();
      expectQueryStringToEqual('?hello=world');

      await goForward();
      expectQueryStringToEqual('?page_size=5');
      expectFirstPage();

      await goForward();
      expectQueryStringToEqual('?offset=5&page_size=5');
      expectSecondPage();

      await goForward();
      expectQueryStringToEqual('?offset=10&page_size=5');
      expectThirdPage();
    });
  }); // end describe('navigation')

  describe('dynamic content', () => {
    let changelistsPageSk;
    beforeEach(async () => {
      changelistsPageSk = await loadChangelistsPageSk();
    });

    it('responds to clicking the show all checkbox', () => {
      fetchMock.get('/json/v1/changelists?offset=0&size=50', empty);
      // click on the input inside the checkbox, otherwise, we see double
      // events, since checkbox-sk "re-throws" the click event.
      const showAllBox = $$('.controls checkbox-sk input', changelistsPageSk);
      expect(showAllBox).to.not.be.null;
      expect(changelistsPageSk._showAll).to.equal(false);
      expectQueryStringToEqual('');
      showAllBox.click();
      expect(changelistsPageSk._showAll).to.equal(true);
      expectQueryStringToEqual('?page_size=50&show_all=true');
      showAllBox.click();
      expect(changelistsPageSk._showAll).to.equal(false);
      expectQueryStringToEqual('?page_size=50');
      showAllBox.click();
      expect(changelistsPageSk._showAll).to.equal(true);
      expectQueryStringToEqual('?page_size=50&show_all=true');
    });
  }); // end describe('dynamic content')
});

function goToNextPage(changelistsPageSk) {
  const event = eventPromise('end-task');
  $$('pagination-sk button.next', changelistsPageSk).click();
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

function expectFirstPage(changelistsPageSk) {
  expect($('td.owner', changelistsPageSk)[0].innerText).to.contain('alpha');
}

function expectSecondPage(changelistsPageSk) {
  expect($('td.owner', changelistsPageSk)[0].innerText).to.contain('zeta');
}

function expectThirdPage(changelistsPageSk) {
  expect($('td.owner', changelistsPageSk)[0].innerText).to.contain('lambda');
}
