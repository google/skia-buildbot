import './index.js';

import { $, $$ } from 'common-sk/modules/dom';
import {
  changelistSummaries_5,
  changelistSummaries_5_offset5,
  changelistSummaries_5_offset10,
  empty
} from './test_data';
import { eventPromise, expectNoUnmatchedCalls } from '../test_util';
import { fetchMock }  from 'fetch-mock';

describe('changelists-page-sk', () => {
  // A reusable HTML element in which we create our element under test.
  const container = document.createElement('div');
  document.body.appendChild(container);

  beforeEach(function() {
    // Clear out any query params we might have to not mess with our current state.
    setQueryString('');
  });

  beforeEach(function() {
    // These are the default offset/page_size params
    fetchMock.get('/json/changelists?offset=0&size=50', JSON.stringify(changelistSummaries_5));

    fetchMock.catch(404);
  });

  afterEach(function() {
    // Completely remove the mocking which allows each test
    // to be able to mess with the mocked routes w/o impacting other tests.
    fetchMock.reset();
  });

  afterEach(function() {
    container.innerHTML = '';
  });

  // calls the test callback with an element under test 'ele'.
  // We can't put the describes inside the whenDefined callback because
  // that doesn't work on Firefox (and possibly other places).
  function createElement(test) {
    return window.customElements.whenDefined('changelists-page-sk').then(() => {
      container.innerHTML = `<changelists-page-sk></changelists-page-sk>`;
      expect(container.firstElementChild).to.not.be.null;
      test(container.firstElementChild);
    });
  }

  function whenPageLoads(test) {
    // The changelists-page-sk emits an 'end-task' event when each fetch finishes.
    // For now, there is only one, but this logic may have to be tweaked if we
    // do multiple.
    let ran = false;
    let ele = null;
    const fn = (e) => {
      e.stopPropagation(); // Prevent interference with eventPromise('end-task').
      // reset for next time
      container.removeEventListener('end-task', fn);
      if (!ran) {
        ran = true; // prevent multiple runs if the test makes the
                    // app go busy (e.g. if it calls fetch).
        test(ele);
      }
    }
    // add the listener and then create the element to make sure we don't miss
    // the busy-end event. The busy-end event should trigger when all fetches
    // are done and the page is rendered.
    container.addEventListener('end-task', fn);
    createElement((e) => {
      ele = e;
    });
  }

  //===============TESTS START====================================

  describe('html layout', () => {
    it('should make a table with 5 rows in the body', (done) => {
      whenPageLoads((ele) => {
        const tbl = $$('table', ele);
        expect(tbl).to.not.be.null;
        const rows = $('tbody tr', ele);
        expect(rows.length).to.equal(5); // one row per item in changelistSummaries_5
        done();
      });
    });

    it('has icons that indicate the status', (done) => {
      whenPageLoads((ele) => {
        const rows = $('tbody tr', ele);
        // First row has an open CL.
        let icon = $$('cached-icon-sk', rows[0]);
        expect(icon).to.not.be.null;
        // Fourth row has an abandoned CL.
        icon = $$('block-icon-sk', rows[3]);
        expect(icon).to.not.be.null;
        // Fifth row has an closed CL.
        icon = $$('done-icon-sk', rows[4]);
        expect(icon).to.not.be.null;
        done();
      });
    });
  }); // end describe('html layout')

  describe('api calls', () => {
    it('includes pagination params in request to changelists', (done) => {
      whenPageLoads((ele) => {
        fetchMock.resetHistory();

        fetchMock.get('/json/changelists?offset=100&size=10', JSON.stringify(empty));
        // pretend these were loaded in via stateReflector
        ele._offset = 100;
        ele._page_size = 10;

        ele._fetch().then(() => {
          expectNoUnmatchedCalls(fetchMock);
          done();
        });
      });
    });
  }); // end describe('api calls')

  describe('navigation', () => {
    it('responds to the browser back/forward buttons', (done) => {
      // First page of results.
      fetchMock.get(
          '/json/changelists?offset=0&size=5',
          JSON.stringify(changelistSummaries_5));
      // Second page of results.
      fetchMock.get(
          '/json/changelists?offset=5&size=5',
          JSON.stringify(changelistSummaries_5_offset5));
      // Third page of results.
      fetchMock.get(
          '/json/changelists?offset=10&size=5',
          JSON.stringify(changelistSummaries_5_offset10));

      // Random query string value before instantiating the component under
      // test. We'll test that we can navigate back to this URL using the
      // browser's back button.
      setQueryString('?hello=world');

      // Query string at component instantiation. This specifies the page size
      // required for the mock RPCs above to work.
      setQueryString('?page_size=5');

      whenPageLoads(async (el) => {
        expectQueryStringToEqual('?page_size=5');
        expectFirstPage();

        await goToNextPage(el);
        expectQueryStringToEqual('?offset=5&page_size=5');
        expectSecondPage();

        await goToNextPage(el);
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

        done();
      });
    });
  }); // end describe('navigation')

  function setQueryString(q) {
    history.pushState(
        null, '', window.location.origin + window.location.pathname + q);
  }

  function goToNextPage(el) {
    const event = eventPromise('end-task');
    $$('pagination-sk button.next', el).click();
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

  function expectQueryStringToEqual(q) {
    expect(window.location.search).to.equal(q);
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
});
