import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import {
  firstPage,
  firstPageAfterUndoingFirstEntry,
  firstPageWithoutDetailsAfterUndoingFirstEntry,
  secondPage,
  thirdPage,
} from './test_data';
import {
  eventPromise,
  expectQueryStringToEqual,
  setQueryString,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';

describe('triagelog-page-sk', () => {
  const newInstance = setUpElementUnderTest('triagelog-page-sk');

  // Instantiate page; wait for RPCs to complete and for the page to render.
  const loadTriagelogPageSk = async () => {
    const event = eventPromise('end-task');
    const triagelogPageSk = newInstance();
    await event;
    return triagelogPageSk;
  };

  beforeEach(async () => {
    // Clear query string before each test case. This is needed for test cases
    // that exercise the stateReflector and the browser's back/forward buttons.
    setQueryString('');
  });

  afterEach(() => {
    expect(fetchMock.done()).to.be.true; // All mock RPCs called at least once.
    // Remove fetch mocking to prevent test cases interfering with each other.
    fetchMock.reset();
  });

  it('shows the right initial entries', async () => {
    fetchMock.get(
      '/json/triagelog?details=true&offset=0&size=20', firstPage,
    );
    const triagelogPageSk = await loadTriagelogPageSk(); // Load first page.
    expectQueryStringToEqual(''); // No state reflected to the URL.
    expectFirstPageOfResults(triagelogPageSk);
  });

  it('advances to the second page of results', async () => {
    fetchMock.get(
      '/json/triagelog?details=true&offset=0&size=20', firstPage,
    );
    fetchMock.get(
      '/json/triagelog?details=true&offset=3&size=3', secondPage,
    );

    const triagelogPageSk = await loadTriagelogPageSk(); // Load first page.
    await goToNextPageOfResults(); // Load second page.
    expectQueryStringToEqual('?offset=3&page_size=3'); // Reflected in URL.
    expectSecondPageOfResults(triagelogPageSk);
  });

  it('undoes an entry', async () => {
    fetchMock.get(
      '/json/triagelog?details=true&offset=0&size=20', firstPage,
    );
    // We mimic the current behavior of the undo RPC, which is to always return
    // the first page of results.
    // TODO(lovisolo): Rethink this after we delete the old triage log page.
    fetchMock.post(
      '/json/triagelog/undo?id=aaa',
      firstPageWithoutDetailsAfterUndoingFirstEntry,
    );
    fetchMock.get(
      '/json/triagelog?details=true&offset=0&size=3',
      firstPageAfterUndoingFirstEntry,
    );

    const triagelogPageSk = await loadTriagelogPageSk(); // Load first page.
    expectFirstPageOfResults(triagelogPageSk);
    await undoFirstEntry(triagelogPageSk);
    expectFirstPageOfResultsFirstEntryUndone(triagelogPageSk);
  });

  it('handles the "issue" URL parameter', async () => {
    fetchMock.get(
      '/json/triagelog?details=true&offset=0&size=20&issue=123456',
      firstPage,
    );
    setQueryString('?issue=123456');
    const triagelogPageSk = await loadTriagelogPageSk(); // Load first page.
    expectQueryStringToEqual('?issue=123456'); // No changes to the URL.
    expectFirstPageOfResults(triagelogPageSk, '123456' /*= changelistID */);
  });

  describe('URL parameters', () => {
    it('initializes paging based on the URL parameters', async () => {
      fetchMock.get(
        '/json/triagelog?details=true&offset=3&size=3', secondPage,
      );

      setQueryString('?offset=3&page_size=3');
      const triagelogPageSk = await loadTriagelogPageSk();
      expectSecondPageOfResults(triagelogPageSk);
    });

    it('responds to back and forward browser buttons', async () => {
      fetchMock.get(
        '/json/triagelog?details=true&offset=0&size=20', firstPage,
      );
      fetchMock.get(
        '/json/triagelog?details=true&offset=3&size=3', secondPage,
      );
      fetchMock.get(
        '/json/triagelog?details=true&offset=6&size=3', thirdPage,
      );

      // Populate window.history by setting the query string to a random value.
      // We'll then test that we can navigate back to this state by using the
      // browser's back button.
      setQueryString('?hello=world');

      // Clear the query string before loading the component. This will be the
      // state at component instantiation. We'll test that the user doesn't get
      // stuck at the state at component creation when pressing the back button.
      setQueryString('');

      const triagelogPageSk = await loadTriagelogPageSk(); // Load first page.
      expectQueryStringToEqual('');
      expectFirstPageOfResults(triagelogPageSk);

      await goToNextPageOfResults(triagelogPageSk);
      expectQueryStringToEqual('?offset=3&page_size=3');
      expectSecondPageOfResults(triagelogPageSk);

      await goToNextPageOfResults(triagelogPageSk);
      expectQueryStringToEqual('?offset=6&page_size=3');
      expectThirdPageOfResults(triagelogPageSk);

      await goBack();
      expectQueryStringToEqual('?offset=3&page_size=3');
      expectSecondPageOfResults(triagelogPageSk);

      // State at component instantiation.
      await goBack();
      expectQueryStringToEqual('');
      expectFirstPageOfResults(triagelogPageSk);

      // State before the component was instantiated.
      await goBack();
      expectQueryStringToEqual('?hello=world');

      await goForward();
      expectQueryStringToEqual('');
      expectFirstPageOfResults(triagelogPageSk);

      await goForward();
      expectQueryStringToEqual('?offset=3&page_size=3');
      expectSecondPageOfResults(triagelogPageSk);

      await goForward();
      expectQueryStringToEqual('?offset=6&page_size=3');
      expectThirdPageOfResults(triagelogPageSk);
    });
  });

  describe('RPC error', () => {
    it('should emit event "fetch-error" on RPC failure', async () => {
      fetchMock.get('glob:*', 500); // Internal server error on any request.

      const event = eventPromise('fetch-error');
      newInstance(); // Instantiate page; fail due to RPC errors.
      await event;

      expectEmptyPage();
    });
  });
});

function goToNextPageOfResults(triagelogPageSk) {
  const event = eventPromise('end-task');
  $$('pagination-sk button.next', triagelogPageSk).click();
  return event;
}

function undoFirstEntry(triagelogPageSk) {
  const event = eventPromise('end-task');
  $$('tbody button.undo', triagelogPageSk).click();
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

function expectEmptyPage(triagelogPageSk) {
  expect($$('tbody', triagelogPageSk).children).to.be.empty;
}

function expectFirstPageOfResults(triagelogPageSk, changelistID = '') {
  expect(nthEntryTimestamp(triagelogPageSk, 0)).to.equal(toLocalDateStr(1572000000000));
  expect(nthEntryAuthor(triagelogPageSk, 0)).to.equal('alpha@google.com');
  expect(nthEntryNumChanges(triagelogPageSk, 0)).to.equal(2);

  expect(nthDetailsRowTestName(triagelogPageSk, 0)).to.equal('async_rescale_and_read_dog_up');
  expect(nthDetailsRowDigest(triagelogPageSk, 0)).to.equal('f16298eb14e19f9230fe81615200561f');
  expect(nthDetailsRowLabel(triagelogPageSk, 0)).to.equal('positive');
  let digestHref = nthDetailsRowDigestHref(triagelogPageSk, 0);
  expect(digestHref).to.contain(
    '/detail?test=async_rescale_and_read_dog_up&digest=f16298eb14e19f9230fe81615200561f',
  );
  if (changelistID) {
    expect(digestHref).to.contain(`&issue=${changelistID}`);
  } else {
    expect(digestHref).not.to.contain('&issue');
  }

  expect(nthEntryTimestamp(triagelogPageSk, 1)).to.equal(toLocalDateStr(1571900000000));
  expect(nthEntryAuthor(triagelogPageSk, 1)).to.equal('beta@google.com');
  expect(nthEntryNumChanges(triagelogPageSk, 1)).to.equal(1);

  expect(nthDetailsRowTestName(triagelogPageSk, 1)).to.equal('async_rescale_and_read_rose');
  expect(nthDetailsRowDigest(triagelogPageSk, 1)).to.equal('35c77280a7d5378033f9bf8f3c755e78');
  expect(nthDetailsRowLabel(triagelogPageSk, 1)).to.equal('positive');
  digestHref = nthDetailsRowDigestHref(triagelogPageSk, 1);
  expect(digestHref).to.contain(
    '/detail?test=async_rescale_and_read_rose&digest=35c77280a7d5378033f9bf8f3c755e78',
  );
  if (changelistID) {
    expect(digestHref).to.contain(`&issue=${changelistID}`);
  } else {
    expect(digestHref).not.to.contain('&issue');
  }

  expect(nthEntryTimestamp(triagelogPageSk, 2)).to.equal(toLocalDateStr(1571800000000));
  expect(nthEntryAuthor(triagelogPageSk, 2)).to.equal('gamma@google.com');
  expect(nthEntryNumChanges(triagelogPageSk, 2)).to.equal(1);

  expect(nthDetailsRowTestName(triagelogPageSk, 2)).to.equal('draw_image_set');
  expect(nthDetailsRowDigest(triagelogPageSk, 2)).to.equal('b788aadee662c2b0390d698cbe68b808');
  expect(nthDetailsRowLabel(triagelogPageSk, 2)).to.equal('positive');
  digestHref = nthDetailsRowDigestHref(triagelogPageSk, 2);
  expect(digestHref).to.contain(
    '/detail?test=draw_image_set&digest=b788aadee662c2b0390d698cbe68b808',
  );
  if (changelistID) {
    expect(digestHref).to.contain(`&issue=${changelistID}`);
  } else {
    expect(digestHref).not.to.contain('&issue');
  }

  expect(nthDetailsRowTestName(triagelogPageSk, 3)).to.equal('filterbitmap_text_7.00pt');
  expect(nthDetailsRowDigest(triagelogPageSk, 3)).to.equal('454b4b547bc6ceb4cdeb3305553be98a');
  expect(nthDetailsRowLabel(triagelogPageSk, 3)).to.equal('positive');
  digestHref = nthDetailsRowDigestHref(triagelogPageSk, 3);
  expect(digestHref).to.contain(
    '/detail?test=filterbitmap_text_7.00pt&digest=454b4b547bc6ceb4cdeb3305553be98a',
  );
  if (changelistID) {
    expect(digestHref).to.contain(`&issue=${changelistID}`);
  } else {
    expect(digestHref).not.to.contain('&issue');
  }
}

// TODO(kjlubick, lovisolo): rewrite these expects* to be more like above.
function expectFirstPageOfResultsFirstEntryUndone(triagelogPageSk) {
  expect(nthEntry(triagelogPageSk, 0)).to.deep.equal(
    [toLocalDateStr(1571900000000), 'beta@google.com', 1],
  );
  expect(nthDetailsRow(triagelogPageSk, 0)).to.deep.equal([
    'draw_image_set',
    'b788aadee662c2b0390d698cbe68b808',
    digestDetailsHref(
      'draw_image_set',
      'b788aadee662c2b0390d698cbe68b808',
    ),
    'positive']);
  expect(nthEntry(triagelogPageSk, 1)).to.deep.equal(
    [toLocalDateStr(1571800000000), 'gamma@google.com', 1],
  );
  expect(nthDetailsRow(triagelogPageSk, 1)).to.deep.equal([
    'filterbitmap_text_7.00pt',
    '454b4b547bc6ceb4cdeb3305553be98a',
    digestDetailsHref(
      'filterbitmap_text_7.00pt',
      '454b4b547bc6ceb4cdeb3305553be98a',
    ),
    'positive']);
  expect(nthEntry(triagelogPageSk, 2)).to.deep.equal(
    [toLocalDateStr(1571700000000), 'delta@google.com', 1],
  );
  expect(nthDetailsRow(triagelogPageSk, 2)).to.deep.equal([
    'filterbitmap_text_10.00pt',
    'fc8392000945e68334c5ccd333b201b3',
    digestDetailsHref(
      'filterbitmap_text_10.00pt',
      'fc8392000945e68334c5ccd333b201b3',
    ),
    'positive']);
}

function expectSecondPageOfResults(triagelogPageSk) {
  expect(nthEntry(triagelogPageSk, 0)).to.deep.equal(
    [toLocalDateStr(1571700000000), 'delta@google.com', 1],
  );
  expect(nthDetailsRow(triagelogPageSk, 0)).to.deep.equal([
    'filterbitmap_text_10.00pt',
    'fc8392000945e68334c5ccd333b201b3',
    digestDetailsHref(
      'filterbitmap_text_10.00pt',
      'fc8392000945e68334c5ccd333b201b3',
    ),
    'positive']);
  expect(nthEntry(triagelogPageSk, 1)).to.deep.equal([
    toLocalDateStr(1571600000000),
    'epsilon@google.com',
    1]);
  expect(nthDetailsRow(triagelogPageSk, 1)).to.deep.equal([
    'filterbitmap_image_mandrill_32.png',
    '7606bfd486f7dfdf299d9d9da8f99c8e',
    digestDetailsHref(
      'filterbitmap_image_mandrill_32.png',
      '7606bfd486f7dfdf299d9d9da8f99c8e',
    ),
    'positive']);
  expect(nthEntry(triagelogPageSk, 2)).to.deep.equal(
    [toLocalDateStr(1571500000000), 'zeta@google.com', 1],
  );
  expect(nthDetailsRow(triagelogPageSk, 2)).to.deep.equal([
    'drawminibitmaprect_aa',
    '95e1b42fcaaff5d0d08b4ed465d79437',
    digestDetailsHref(
      'drawminibitmaprect_aa',
      '95e1b42fcaaff5d0d08b4ed465d79437',
    ),
    'positive']);
}

function expectThirdPageOfResults(triagelogPageSk) {
  expect(nthEntry(triagelogPageSk, 0)).to.deep.equal(
    [toLocalDateStr(1571400000000), 'eta@google.com', 1],
  );
  expect(nthDetailsRow(triagelogPageSk, 0)).to.deep.equal([
    'colorcomposefilter_wacky',
    '68e41c7f7d91f432fd36d71fe1249443',
    digestDetailsHref(
      'colorcomposefilter_wacky',
      '68e41c7f7d91f432fd36d71fe1249443',
    ),
    'positive']);
  expect(nthEntry(triagelogPageSk, 1)).to.deep.equal(
    [toLocalDateStr(1571300000000), 'theta@google.com', 1],
  );
  expect(nthDetailsRow(triagelogPageSk, 1)).to.deep.equal([
    'circular_arc_stroke_matrix',
    'c482098318879e7d2cf4f0414b607156',
    digestDetailsHref(
      'circular_arc_stroke_matrix',
      'c482098318879e7d2cf4f0414b607156',
    ),
    'positive']);
  expect(nthEntry(triagelogPageSk, 2)).to.deep.equal(
    [toLocalDateStr(1571200000000), 'iota@google.com', 1],
  );
  expect(nthDetailsRow(triagelogPageSk, 2)).to.deep.equal([
    'dftext_blob_persp',
    'a41baae99abd37d9ed606e8bc27df6a2',
    digestDetailsHref(
      'dftext_blob_persp',
      'a41baae99abd37d9ed606e8bc27df6a2',
    ),
    'positive']);
}

const toLocalDateStr = (timestampMS) => new Date(timestampMS).toLocaleString();
const digestDetailsHref = (test, digest) => `${window.location.origin
}/detail?test=${encodeURIComponent(test)}&digest=${encodeURIComponent(digest)}`;

const nthEntryTimestamp = (triagelogPageSk, n) => $('.timestamp', triagelogPageSk)[n].innerText;
const nthEntryAuthor = (triagelogPageSk, n) => $('.author', triagelogPageSk)[n].innerText;
const nthEntryNumChanges = (triagelogPageSk, n) => +$('.num-changes', triagelogPageSk)[n].innerText;
const nthEntry = (triagelogPageSk, n) => [
  nthEntryTimestamp(triagelogPageSk, n),
  nthEntryAuthor(triagelogPageSk, n),
  nthEntryNumChanges(triagelogPageSk, n),
];

const nthDetailsRowTestName = (triagelogPageSk, n) => $('.details .test-name', triagelogPageSk)[n].innerText;
const nthDetailsRowDigest = (triagelogPageSk, n) => $('.details .digest', triagelogPageSk)[n].innerText;
const nthDetailsRowDigestHref = (triagelogPageSk, n) => $('.details .digest a', triagelogPageSk)[n].href;
const nthDetailsRowLabel = (triagelogPageSk, n) => $('.details .label', triagelogPageSk)[n].innerText;
const nthDetailsRow = (triagelogPageSk, n) => [
  nthDetailsRowTestName(triagelogPageSk, n),
  nthDetailsRowDigest(triagelogPageSk, n),
  nthDetailsRowDigestHref(triagelogPageSk, n),
  nthDetailsRowLabel(triagelogPageSk, n),
];

// TODO(lovisolo): Add a function eventSequencePromise() to test_util.js that
//                 takes a list of event names and returns a promise that
//                 resolves when the events are caught in the given sequence,
//                 or reject if any of the events are caught out-of-sequence.
//                 Leverage eventPromise() to implement this.
//
// TODO(lovisolo): Use eventSequencePromise(['begin-task', 'end-task']) above
//                 where appropriate. Idem with 'fetch-error'.
