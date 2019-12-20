import './index.js'

import { $, $$ } from 'common-sk/modules/dom'
import {
  firstPage,
  firstPageAfterUndoingFirstEntry,
  firstPageWithoutDetailsAfterUndoingFirstEntry,
  secondPage,
  thirdPage,
} from './test_data'
import { eventPromise, expectNoUnmatchedCalls } from '../test_util'
import { fetchMock } from 'fetch-mock';

fetchMock.config.overwriteRoutes = true;

describe('triagelog-page-sk', () => {
  // Component under test.
  let triagelogPageSk;

  // Creates a new instance of the component under test.
  function newTriagelogPageSk() {
    triagelogPageSk = document.createElement('triagelog-page-sk');
    document.body.appendChild(triagelogPageSk);
  }

  // Same as newTriagelogPageSk, but returns a promise that will resolve when
  // the page finishes loading.
  function loadTriagelogPageSk() {
    const event = eventPromise('end-task');
    newTriagelogPageSk();
    return event;
  }

  beforeEach(async () => {
    // Clear query string before each test case. This is needed for test cases
    // that exercise the stateReflector and the browser's back/forward buttons.
    setQueryString('');
  });

  afterEach(() => {
    // Remove the stale instance under test.
    if (triagelogPageSk) {
      document.body.removeChild(triagelogPageSk);
      triagelogPageSk = null;
    }
    expect(fetchMock.done()).to.be.true;  // All mock RPCs called at least once.
    expectNoUnmatchedCalls(fetchMock);
    // Remove fetch mocking to prevent test cases interfering with each other.
    fetchMock.reset();
  });

  it('shows the right initial entries', async () => {
    fetchMock.get(
        '/json/triagelog?details=true&offset=0&size=20', firstPage);
    await loadTriagelogPageSk(); // Load first page of results by default.
    expectQueryStringToEqual(''); // No state reflected to the URL.
    expectFirstPageOfResults();
  });

  it('advances to the second page of results', async () => {
    fetchMock.get(
        '/json/triagelog?details=true&offset=0&size=20', firstPage);
    fetchMock.get(
        '/json/triagelog?details=true&offset=3&size=3', secondPage);

    await loadTriagelogPageSk();   // Load first page of results by default.
    await goToNextPageOfResults(); // Load second page.
    expectQueryStringToEqual('?offset=3&page_size=3'); // Reflected in URL.
    expectSecondPageOfResults();
  });

  it('undoes an entry', async () => {
    fetchMock.get(
        '/json/triagelog?details=true&offset=0&size=20', firstPage);
    // We mimic the current behavior of the undo RPC, which is to always return
    // the first page of results.
    // TODO(lovisolo): Rethink this after we delete the old triage log page.
    fetchMock.post(
        '/json/triagelog/undo?id=aaa',
        firstPageWithoutDetailsAfterUndoingFirstEntry);
    fetchMock.get(
        '/json/triagelog?details=true&offset=0&size=3',
        firstPageAfterUndoingFirstEntry);

    await loadTriagelogPageSk(); // Load first page of results by default.
    expectFirstPageOfResults();
    await undoFirstEntry();
    expectFirstPageOfResultsFirstEntryUndone();
  });

  it('handles the "issue" URL parameter', async () => {
    fetchMock.get(
        '/json/triagelog?details=true&offset=0&size=20&issue=123456',
        firstPage);
    setQueryString('?issue=123456')
    await loadTriagelogPageSk(); // Load first page of results by default.
    expectQueryStringToEqual('?issue=123456'); // No changes to the URL.
    expectFirstPageOfResults();
  });

  describe('URL parameters', () => {
    it('initializes paging based on the URL parameters', async () => {
      fetchMock.get(
          '/json/triagelog?details=true&offset=3&size=3', secondPage);

      setQueryString('?offset=3&page_size=3');
      await loadTriagelogPageSk();
      expectSecondPageOfResults();
    });

    it('responds to back and forward browser buttons', async () => {
      fetchMock.get(
          '/json/triagelog?details=true&offset=0&size=20', firstPage);
      fetchMock.get(
          '/json/triagelog?details=true&offset=3&size=3', secondPage);
      fetchMock.get(
          '/json/triagelog?details=true&offset=6&size=3', thirdPage);

      // Populate window.history by setting the query string to a random value.
      // We'll then test that we can navigate back to this state by using the
      // browser's back button.
      setQueryString('?hello=world');

      // Clear the query string before loading the component. This will be the
      // state at component instantiation. We'll test that the user doesn't get
      // stuck at the state at component creation when pressing the back button.
      setQueryString('');

      await loadTriagelogPageSk(); // Load first page of results by default.
      expectQueryStringToEqual('');
      expectFirstPageOfResults();

      await goToNextPageOfResults();
      expectQueryStringToEqual('?offset=3&page_size=3');
      expectSecondPageOfResults();

      await goToNextPageOfResults();
      expectQueryStringToEqual('?offset=6&page_size=3');
      expectThirdPageOfResults();

      await goBack();
      expectQueryStringToEqual('?offset=3&page_size=3');
      expectSecondPageOfResults();

      // State at component instantiation.
      await goBack();
      expectQueryStringToEqual('');
      expectFirstPageOfResults();

      // State before the component was instantiated.
      await goBack();
      expectQueryStringToEqual('?hello=world');

      await goForward();
      expectQueryStringToEqual('');
      expectFirstPageOfResults();

      await goForward();
      expectQueryStringToEqual('?offset=3&page_size=3');
      expectSecondPageOfResults();

      await goForward();
      expectQueryStringToEqual('?offset=6&page_size=3');
      expectThirdPageOfResults();
    });
  });

  describe('RPC error', () => {
    it('should emit event "fetch-error" on RPC failure', async () => {
      fetchMock.get('glob:*', 500);  // Internal server error on any request.

      const event = eventPromise('fetch-error');
      newTriagelogPageSk();
      await event;

      expectEmptyPage();
    });
  });

  function setQueryString(string) {
    history.pushState(
        null,
        '',
        window.location.origin + window.location.pathname + string);
  }

  function goToNextPageOfResults() {
    const event = eventPromise('end-task');
    qq('pagination-sk button.next').click();
    return event;
  }

  function undoFirstEntry() {
    const event = eventPromise('end-task');
    qq('tbody button.undo').click();
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

  function expectQueryStringToEqual(expected) {
    expect(window.location.search).to.equal(expected);
  }

  function expectEmptyPage() {
    expect($$('tbody').children).to.be.empty;
  }

  function expectFirstPageOfResults() {
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
  }

  function expectFirstPageOfResultsFirstEntryUndone() {
    expect(nthEntry(0)).to.deep.equal(
        [toLocalDateStr(1571900000000), 'beta@google.com', 1]);
    expect(nthDetailsRow(0)).to.deep.equal([
      'draw_image_set',
      'b788aadee662c2b0390d698cbe68b808',
      digestDetailsHref(
          'draw_image_set',
          'b788aadee662c2b0390d698cbe68b808'),
      'positive']);
    expect(nthEntry(1)).to.deep.equal(
        [toLocalDateStr(1571800000000), 'gamma@google.com', 1]);
    expect(nthDetailsRow(1)).to.deep.equal([
      'filterbitmap_text_7.00pt',
      '454b4b547bc6ceb4cdeb3305553be98a',
      digestDetailsHref(
          'filterbitmap_text_7.00pt',
          '454b4b547bc6ceb4cdeb3305553be98a'),
      'positive']);
    expect(nthEntry(2)).to.deep.equal(
        [toLocalDateStr(1571700000000), 'delta@google.com', 1]);
    expect(nthDetailsRow(2)).to.deep.equal([
      'filterbitmap_text_10.00pt',
      'fc8392000945e68334c5ccd333b201b3',
      digestDetailsHref(
          'filterbitmap_text_10.00pt',
          'fc8392000945e68334c5ccd333b201b3'),
      'positive']);
  }

  function expectSecondPageOfResults() {
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
  }

  function expectThirdPageOfResults() {
    expect(nthEntry(0)).to.deep.equal(
        [toLocalDateStr(1571400000000), 'eta@google.com', 1]);
    expect(nthDetailsRow(0)).to.deep.equal([
      'colorcomposefilter_wacky',
      '68e41c7f7d91f432fd36d71fe1249443',
      digestDetailsHref(
          'colorcomposefilter_wacky',
          '68e41c7f7d91f432fd36d71fe1249443'),
      'positive']);
    expect(nthEntry(1)).to.deep.equal(
        [toLocalDateStr(1571300000000), 'theta@google.com', 1]);
    expect(nthDetailsRow(1)).to.deep.equal([
      'circular_arc_stroke_matrix',
      'c482098318879e7d2cf4f0414b607156',
      digestDetailsHref(
          'circular_arc_stroke_matrix',
          'c482098318879e7d2cf4f0414b607156'),
      'positive']);
    expect(nthEntry(2)).to.deep.equal(
        [toLocalDateStr(1571200000000), 'iota@google.com', 1]);
    expect(nthDetailsRow(2)).to.deep.equal([
      'dftext_blob_persp',
      'a41baae99abd37d9ed606e8bc27df6a2',
      digestDetailsHref(
          'dftext_blob_persp',
          'a41baae99abd37d9ed606e8bc27df6a2'),
      'positive']);
  }

  // Convenience functions to query child DOM nodes of the component under test.
  const q = (str) => $(str, triagelogPageSk);
  const qq = (str) => $$(str, triagelogPageSk);

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

  // TODO(lovisolo): Add a function eventSequencePromise() to test_util.js that
  //                 takes a list of event names and returns a promise that
  //                 resolves when the events are caught in the given sequence,
  //                 or reject if any of the events are caught out-of-sequence.
  //                 Leverage eventPromise() to implement this.
  //
  // TODO(lovisolo): Use eventSequencePromise(['begin-task', 'end-task']) above
  //                 where appropriate. Idem with 'fetch-error'.

});
