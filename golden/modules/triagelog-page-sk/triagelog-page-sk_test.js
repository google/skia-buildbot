import './index.js'

import { $, $$ } from 'common-sk/modules/dom'
import {
  firstPage,
  firstPageAfterUndoingFirstEntry,
  firstPageWithDetails,
  secondPage,
  secondPageWithDetails
} from './test_data'
import { expectNoUnmatchedCalls } from '../test_util'
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

  // Convenience functions to query DOM elements in the component under test.
  let q = (str) => $(str, triagelogPageSk);
  let qq = (str) => $$(str, triagelogPageSk);

  beforeEach(async () => {
    // The test runner will pollute the URL with its own query string, so we
    // need to clean it before running our test cases, as it could interfere
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

      // Load first page.
      await endTaskEvent(newTriagelogPageSk);

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

      // Load first page.
      await endTaskEvent(newTriagelogPageSk);

      // Advance to the next page.
      await endTaskEvent(() => qq('pagination-sk button.next').click());

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

      // Load first page.
      await endTaskEvent(newTriagelogPageSk);

      // Undo first entry.
      await endTaskEvent(() => qq('tbody button.undo').click());

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

      // Load first page.
      await endTaskEvent(newTriagelogPageSk);

      // Show details.
      await endTaskEvent(() => qq('.details-checkbox').click());

      // Assert that we get the first page of triagelog entries with details.
      expect(nthEntry(0)).to.deep.equal(
          [toLocalDateStr(1572000000000), 'alpha@google.com', 2]);
      expect(nthDetailsRow(0)).to.deep.equal(
          ['async_rescale_and_read_dog_up', 'f16298eb14e19f9230fe81615200561f',
            'positive']);
      expect(nthDetailsRow(1)).to.deep.equal(
          ['async_rescale_and_read_rose', '35c77280a7d5378033f9bf8f3c755e78',
            'positive']);
      expect(nthEntry(1)).to.deep.equal(
          [toLocalDateStr(1571900000000), 'beta@google.com', 1]);
      expect(nthDetailsRow(2)).to.deep.equal(
          ['draw_image_set', 'b788aadee662c2b0390d698cbe68b808', 'positive']);
      expect(nthEntry(2)).to.deep.equal(
          [toLocalDateStr(1571800000000), 'gamma@google.com', 1]);
      expect(nthDetailsRow(3)).to.deep.equal(
          ['filterbitmap_text_7.00pt', '454b4b547bc6ceb4cdeb3305553be98a',
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

      // Load first page.
      await endTaskEvent(newTriagelogPageSk);

      // Show details.
      await endTaskEvent(() => qq('.details-checkbox').click());

      // Advance to the next page.
      await endTaskEvent(() => qq('pagination-sk button.next').click());

      // Assert that we get the second page of triagelog entries with details.
      expect(nthEntry(0)).to.deep.equal(
          [toLocalDateStr(1571700000000), 'delta@google.com', 1]);
      expect(nthDetailsRow(0)).to.deep.equal(
          ['filterbitmap_text_10.00pt', 'fc8392000945e68334c5ccd333b201b3',
            'positive']);
      expect(nthEntry(1)).to.deep.equal(
          [toLocalDateStr(1571600000000), 'epsilon@google.com', 1]);
      expect(nthDetailsRow(1)).to.deep.equal(['filterbitmap_image_mandrill_32.png',
        '7606bfd486f7dfdf299d9d9da8f99c8e', 'positive']);
      expect(nthEntry(2)).to.deep.equal(
          [toLocalDateStr(1571500000000), 'zeta@google.com', 1]);
      expect(nthDetailsRow(2)).to.deep.equal(
          ['drawminibitmaprect_aa', '95e1b42fcaaff5d0d08b4ed465d79437',
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

      // Load first page.
      await endTaskEvent(newTriagelogPageSk);

      // Show details.
      await endTaskEvent(() => qq('.details-checkbox').click());

      // Undo first entry.
      await endTaskEvent(() => qq('tbody button.undo').click());

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

  let toLocalDateStr = (timestampMS) => new Date(timestampMS).toLocaleString();

  let nthEntryTimestamp = (n) => q('.timestamp')[n].innerText;
  let nthEntryAuthor = (n) => q('.author')[n].innerText;
  let nthEntryNumChanges = (n) => +q('.num-changes')[n].innerText;
  let nthEntry = (n) => [
    nthEntryTimestamp(n),
    nthEntryAuthor(n),
    nthEntryNumChanges(n)
  ];

  let nthDetailsRowTestName = (n) => q('.details .test-name')[n].innerText;
  let nthDetailsRowDigest = (n) => q('.details .digest')[n].innerText;
  let nthDetailsRowLabel = (n) => q('.details .label')[n].innerText;
  let nthDetailsRow = (n) => [
    nthDetailsRowTestName(n),
    nthDetailsRowDigest(n),
    nthDetailsRowLabel(n)
  ];

  // Adds an event handler for "end-task", then calls the given function.
  // Returns a promise that will resolve when the "end-task" event is caught.
  //
  // If the "end-task" event is not caught within 5 seconds, the returned
  // promise will be rejected. In either case the event handler will be removed.
  function endTaskEvent(fn) {
    const timeoutMillis = 5000;

    // The executor function passed as a constructor argument to the Promise
    // object is executed immediately.
    let promise = new Promise((resolve, reject) => {
      let handler = (e) => {
        document.body.removeEventListener('end-task', handler);
        clearTimeout(timeout);
        resolve(e);
      };
      let timeout = setTimeout(() => {
        document.body.removeEventListener('end-task', handler);
        reject(new Error(`timed out after ${timeoutMillis} milliseconds`));
      }, timeoutMillis);
      document.body.addEventListener('end-task', handler);
    });

    // At this point we're guaranteed to have an event listener for "end-task".
    fn();

    return promise;
  }
});
