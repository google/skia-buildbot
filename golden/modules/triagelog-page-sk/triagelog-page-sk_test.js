import './index.js'

import { $, $$ } from 'common-sk/modules/dom'
import {
  firstPage,
  firstPageAfterUndoingFirstEntry,
  firstPageWithDetails,
  secondPage,
  secondPageWithDetails
} from './test_data'
import { eventPromise, expectNoUnmatchedCalls } from '../test_util'
import { fetchMock } from 'fetch-mock';

fetchMock.config.overwriteRoutes = true;

describe('triagelog-page-sk', () => {
  // Component under test.
  let triagelogPageSk;

  // Each test case is responsible for instantiating its component under test.
  function newTriagelogPageSk() {
    triagelogPageSk = document.createElement('triagelog-page-sk');
    document.body.appendChild(triagelogPageSk);
  }

  // Convenience functions to query child DOM nodes of the component under test.
  const q = (str) => $(str, triagelogPageSk);
  const qq = (str) => $$(str, triagelogPageSk);

  beforeEach(async () => {
    // The test runner will pollute the URL with its own query string, so we
    // need to clean it before running our test cases, as this could interfere
    // with the stateReflector.
    history.pushState(
        null,
        '',
        window.location.origin + window.location.pathname + '?');

    // Make sure the component under test is defined before carrying on.
    await window.customElements.whenDefined('triagelog-page-sk');
  });

  afterEach(() => {
    // Remove the stale instance under test.
    if (triagelogPageSk) {
      document.body.removeChild(triagelogPageSk);
      triagelogPageSk = null;
    }

    // Remove fetch mocking to prevent test cases interfering with each other.
    fetchMock.reset();
  });

  describe('details hidden', () => {
    it('shows the right initial entries', async () => {
      fetchMock.get(
          '/json/triagelog?details=false&offset=0&size=20',
          firstPage);

      // Instantiate the page. This retrieves the first page of triage logs.
      await endTaskEvent(newTriagelogPageSk);

      // Assert that the query string was updated with the pagination parameters
      // returned by the mock RPC.
      expect(window.location.search).to.equal("?page_size=3");

      // Assert that we get the first page of triagelog entries.
      expect(nthEntry(0)).to.deep.equal(
          [toLocalDateStr(1572000000000), 'alpha@google.com', 2]);
      expect(nthEntry(1)).to.deep.equal(
          [toLocalDateStr(1571900000000), 'beta@google.com', 1]);
      expect(nthEntry(2)).to.deep.equal(
          [toLocalDateStr(1571800000000), 'gamma@google.com', 1]);

      // Assert that no details are visible.
      expect(q('.details .test-name')).to.be.empty;

      expect(fetchMock.done()).to.be.true;
      expectNoUnmatchedCalls(fetchMock);
    });

    it('loads the next page', async () => {
      fetchMock.get(
          '/json/triagelog?details=false&offset=0&size=20',
          firstPage);
      fetchMock.get(
          '/json/triagelog?details=false&offset=3&size=3',
          secondPage);

      // Instantiate the page. This retrieves the first page of triage logs.
      await endTaskEvent(newTriagelogPageSk);

      // Advance to the next page.
      await endTaskEvent(() => qq('pagination-sk button.next').click());

      // Assert that the query string was updated with the pagination parameters
      // returned by the mock RPC.
      expect(window.location.search).to.equal("?offset=3&page_size=3");

      // Assert that we get the second page of triagelog entries.
      expect(nthEntry(0)).to.deep.equal(
          [toLocalDateStr(1571700000000), 'delta@google.com', 1]);
      expect(nthEntry(1)).to.deep.equal(
          [toLocalDateStr(1571600000000), 'epsilon@google.com', 1]);
      expect(nthEntry(2)).to.deep.equal(
          [toLocalDateStr(1571500000000), 'zeta@google.com', 1]);

      // Assert that no details are visible.
      expect(q('.details .test-name')).to.be.empty;

      expect(fetchMock.done()).to.be.true;
      expectNoUnmatchedCalls(fetchMock);
    });

    it('undoes an entry', async () => {
      fetchMock.get(
          '/json/triagelog?details=false&offset=0&size=20',
          firstPage);
      fetchMock.post(
          '/json/triagelog/undo?id=aaa',
          firstPageAfterUndoingFirstEntry);

      // Instantiate the page. This retrieves the first page of triage logs.
      await endTaskEvent(newTriagelogPageSk);

      // Undo first entry.
      await endTaskEvent(() => qq('tbody button.undo').click());

      // Assert that the query string was updated with the pagination parameters
      // returned by the mock RPC.
      expect(window.location.search).to.equal("?page_size=3");

      // Assert that we get the first page of triagelog entries minus the undone
      // one.
      expect(nthEntry(0)).to.deep.equal(
          [toLocalDateStr(1571900000000), 'beta@google.com', 1]);
      expect(nthEntry(1)).to.deep.equal(
          [toLocalDateStr(1571800000000), 'gamma@google.com', 1]);
      expect(nthEntry(2)).to.deep.equal(
          [toLocalDateStr(1571700000000), 'delta@google.com', 1]);

      // Assert that no details are visible.
      expect(q('.details .test-name')).to.be.empty;

      expect(fetchMock.done()).to.be.true;
      expectNoUnmatchedCalls(fetchMock);
    });
  });

  describe('details visible', () => {
    it('shows details when checkbox is clicked', async () => {
      fetchMock.get(
          '/json/triagelog?details=false&offset=0&size=20',
          firstPage);
      fetchMock.get(
          '/json/triagelog?details=true&offset=0&size=3',
          firstPageWithDetails);

      // Instantiate the page. This retrieves the first page of triage logs.
      await endTaskEvent(newTriagelogPageSk);

      // Show details.
      await endTaskEvent(() => qq('.details-checkbox').click());

      // Assert that the query string was updated with the pagination parameters
      // returned by the mock RPC.
      expect(window.location.search).to.equal("?details=true&page_size=3");

      // Assert that we get the first page of triagelog entries with details.
      expect(nthEntry(0)).to.deep.equal(
          [toLocalDateStr(1572000000000), 'alpha@google.com', 2]);
      expect(nthDetailsRow(0)).to.deep.equal([
          'async_rescale_and_read_dog_up',
          'f16298eb14e19f9230fe81615200561f',
          digestDetailsHref(
              'async_rescale_and_read_dog_up',
              'f16298eb14e19f9230fe81615200561f'),
          'positive']);
      expect(nthDetailsRow(1)).to.deep.equal([
          'async_rescale_and_read_rose',
          '35c77280a7d5378033f9bf8f3c755e78',
          digestDetailsHref(
              'async_rescale_and_read_rose',
              '35c77280a7d5378033f9bf8f3c755e78'),
          'positive']);
      expect(nthEntry(1)).to.deep.equal(
          [toLocalDateStr(1571900000000), 'beta@google.com', 1]);
      expect(nthDetailsRow(2)).to.deep.equal([
          'draw_image_set',
          'b788aadee662c2b0390d698cbe68b808',
          digestDetailsHref(
              'draw_image_set',
              'b788aadee662c2b0390d698cbe68b808'),
          'positive']);
      expect(nthEntry(2)).to.deep.equal(
          [toLocalDateStr(1571800000000), 'gamma@google.com', 1]);
      expect(nthDetailsRow(3)).to.deep.equal([
          'filterbitmap_text_7.00pt',
          '454b4b547bc6ceb4cdeb3305553be98a',
          digestDetailsHref(
              'filterbitmap_text_7.00pt',
              '454b4b547bc6ceb4cdeb3305553be98a'),
          'positive']);

      expect(fetchMock.done()).to.be.true;
      expectNoUnmatchedCalls(fetchMock);
    });

    it('details remain visible when loading next page', async () => {
      fetchMock.get(
          '/json/triagelog?details=false&offset=0&size=20',
          firstPage);
      fetchMock.get(
          '/json/triagelog?details=true&offset=0&size=3',
          firstPageWithDetails);
      fetchMock.get(
          '/json/triagelog?details=true&offset=3&size=3',
          secondPageWithDetails);

      // Instantiate the page. This retrieves the first page of triage logs.
      await endTaskEvent(newTriagelogPageSk);

      // Show details.
      await endTaskEvent(() => qq('.details-checkbox').click());

      // Advance to the next page.
      await endTaskEvent(() => qq('pagination-sk button.next').click());

      // Assert that the query string was updated with the pagination parameters
      // returned by the mock RPC.
      expect(window.location.search).to.equal(
          "?details=true&offset=3&page_size=3");

      // Assert that we get the second page of triagelog entries with details.
      expect(nthEntry(0)).to.deep.equal(
          [toLocalDateStr(1571700000000), 'delta@google.com', 1]);
      expect(nthDetailsRow(0)).to.deep.equal([
          'filterbitmap_text_10.00pt',
          'fc8392000945e68334c5ccd333b201b3',
          digestDetailsHref(
              'filterbitmap_text_10.00pt',
              'fc8392000945e68334c5ccd333b201b3'),
          'positive']);
      expect(nthEntry(1)).to.deep.equal([
          toLocalDateStr(1571600000000),
          'epsilon@google.com',
          1]);
      expect(nthDetailsRow(1)).to.deep.equal([
          'filterbitmap_image_mandrill_32.png',
          '7606bfd486f7dfdf299d9d9da8f99c8e',
          digestDetailsHref(
              'filterbitmap_image_mandrill_32.png',
              '7606bfd486f7dfdf299d9d9da8f99c8e'),
          'positive']);
      expect(nthEntry(2)).to.deep.equal(
          [toLocalDateStr(1571500000000), 'zeta@google.com', 1]);
      expect(nthDetailsRow(2)).to.deep.equal([
          'drawminibitmaprect_aa',
          '95e1b42fcaaff5d0d08b4ed465d79437',
          digestDetailsHref(
              'drawminibitmaprect_aa',
              '95e1b42fcaaff5d0d08b4ed465d79437'),
          'positive']);

      expect(fetchMock.done()).to.be.true;
      expectNoUnmatchedCalls(fetchMock);
    });

    it('undoes an entry, which unchecks "Show details"', async () => {
      fetchMock.get(
          '/json/triagelog?details=false&offset=0&size=20',
          firstPage);
      fetchMock.get(
          '/json/triagelog?details=true&offset=0&size=3',
          firstPageWithDetails);
      fetchMock.post(
          '/json/triagelog/undo?id=aaa',
          firstPageAfterUndoingFirstEntry);

      // Instantiate the page. This retrieves the first page of triage logs.
      await endTaskEvent(newTriagelogPageSk);

      // Show details.
      await endTaskEvent(() => qq('.details-checkbox').click());

      // Undo first entry.
      await endTaskEvent(() => qq('tbody button.undo').click());

      // Assert that the query string was updated with the pagination parameters
      // returned by the mock RPC.
      expect(window.location.search).to.equal("?page_size=3");

      // "Show details" should be unchecked now.
      expect(qq('.details-checkbox').checked).to.be.false;

      // Assert that we get the first page of triagelog entries.
      expect(nthEntry(0)).to.deep.equal(
          [toLocalDateStr(1571900000000), 'beta@google.com', 1]);
      expect(nthEntry(1)).to.deep.equal(
          [toLocalDateStr(1571800000000), 'gamma@google.com', 1]);
      expect(nthEntry(2)).to.deep.equal(
          [toLocalDateStr(1571700000000), 'delta@google.com', 1]);

      // Assert that no details are visible.
      expect(q('.details .test-name')).to.be.empty;

      expect(fetchMock.done()).to.be.true;
      expectNoUnmatchedCalls(fetchMock);
    });
  });

  describe('URL parameters', () => {
    it('initializes paging based on the URL parameters', async () => {
      // Query string set to second page of results, with details.
      const queryString = '?details=true&offset=3&page_size=3';
      history.pushState(
          null,
          '',
          window.location.origin + window.location.pathname + queryString);

      fetchMock.get(
          '/json/triagelog?details=true&offset=3&size=3',
          secondPageWithDetails);

      // Instantiate the page. This retrieves the first page of triage logs.
      await endTaskEvent(newTriagelogPageSk);

      // "Show details" should be checked.
      expect(qq('.details-checkbox').checked).to.be.true;

      // Assert that we get the second page of triagelog entries with details.
      expect(nthEntry(0)).to.deep.equal(
          [toLocalDateStr(1571700000000), 'delta@google.com', 1]);
      expect(nthDetailsRow(0)).to.deep.equal([
          'filterbitmap_text_10.00pt',
          'fc8392000945e68334c5ccd333b201b3',
          digestDetailsHref(
              'filterbitmap_text_10.00pt',
              'fc8392000945e68334c5ccd333b201b3'),
          'positive']);
      expect(nthEntry(1)).to.deep.equal(
          [toLocalDateStr(1571600000000), 'epsilon@google.com', 1]);
      expect(nthDetailsRow(1)).to.deep.equal([
          'filterbitmap_image_mandrill_32.png',
          '7606bfd486f7dfdf299d9d9da8f99c8e',
          digestDetailsHref(
              'filterbitmap_image_mandrill_32.png',
              '7606bfd486f7dfdf299d9d9da8f99c8e'),
          'positive']);
      expect(nthEntry(2)).to.deep.equal(
          [toLocalDateStr(1571500000000), 'zeta@google.com', 1]);
      expect(nthDetailsRow(2)).to.deep.equal([
          'drawminibitmaprect_aa',
          '95e1b42fcaaff5d0d08b4ed465d79437',
          digestDetailsHref(
              'drawminibitmaprect_aa',
              '95e1b42fcaaff5d0d08b4ed465d79437'),
          'positive']);

      expect(fetchMock.done()).to.be.true;
      expectNoUnmatchedCalls(fetchMock);
    });
  });

  describe('RPC error', () => {
    it('should emit event "fetch-error" on RPC failure', async () => {
      fetchMock.get('glob:*', 500);  // Internal server error on any request.

      // Instantiate the page. This will try retrieving the first page of
      // results and fail with an internal server error.
      await fetchErrorEvent(newTriagelogPageSk);

      // At this point it's guaranteed that the fetch-error event was caught.
      // Assert that there are no log entries on screen.
      expect($$('tbody').children).to.be.empty;
    });
  });

  const toLocalDateStr = (timestampMS) => new Date(timestampMS).toLocaleString();
  const digestDetailsHref = (test, digest) =>
      window.location.origin +
          `/detail?test=${encodeURIComponent(test)}&digest=${encodeURIComponent(digest)}`;

  const nthEntryTimestamp = (n) => q('.timestamp')[n].innerText;
  const nthEntryAuthor = (n) => q('.author')[n].innerText;
  const nthEntryNumChanges = (n) => +q('.num-changes')[n].innerText;
  const nthEntry = (n) => [
    nthEntryTimestamp(n),
    nthEntryAuthor(n),
    nthEntryNumChanges(n)
  ];

  const nthDetailsRowTestName =
      (n) => q('.details .test-name')[n].innerText;
  const nthDetailsRowDigest = (n) => q('.details .digest')[n].innerText;
  const nthDetailsRowDigestHref = (n) => q('.details .digest a')[n].href;
  const nthDetailsRowLabel = (n) => q('.details .label')[n].innerText;
  const nthDetailsRow = (n) => [
      nthDetailsRowTestName(n),
      nthDetailsRowDigest(n),
      nthDetailsRowDigestHref(n),
      nthDetailsRowLabel(n)
  ];

  // TODO(lovisolo): Add function to build a promise that will resolve when the
  //                 given sequence of events is caught.
  //
  // TODO(lovisolo): Use said function to assert on the tests above that event
  //                 "begin-task" always precedes events "end-task" and
  //                 "fetch-error".
  //
  // TODO(lovisolo): Move said promise builder functions to test_util.js.

  // Adds an event handler for "end-task", then calls the given function.
  // Returns a promise that will resolve when the event is caught.
  //
  // The promise will be rejected if the event isn't caught within 5 seconds.
  function endTaskEvent(fn) {
    const promise = eventPromise('end-task');
    fn();
    return promise;
  }

  // Adds an event handler for "fetch-error", then calls the given function.
  // Returns a promise that will resolve when the event is caught.
  //
  // The promise will be rejected if the event isn't caught within 5 seconds.
  function fetchErrorEvent(fn) {
    const promise = eventPromise('fetch-error');
    fn();
    return promise;
  }

});
