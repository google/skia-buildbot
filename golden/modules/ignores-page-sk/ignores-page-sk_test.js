import './index.js';

import { $, $$ } from 'common-sk/modules/dom';
import { eventPromise, expectNoUnmatchedCalls } from "../test_util";
import { fakeNow, ignoreRules_10 } from './test_data';
import { fetchMock }  from 'fetch-mock';

function setQueryString(q) {
  history.pushState(
    null, '', window.location.origin + window.location.pathname + q);
}

describe('ignores-page-sk', () => {
  const regularNow = Date.now;
  let ignoresSk;

  beforeEach(async function () {
    // Clear out any query params we might have to not mess with our current state.
    setQueryString('');
    // These are the default offset/page_size params
    fetchMock.get('/json/ignores?counts=1', JSON.stringify(ignoreRules_10));
    // set the time to our mocked Now
    Date.now = () => fakeNow;

    const event = eventPromise('end-task');
    ignoresSk = document.createElement('ignores-page-sk');
    document.body.appendChild(ignoresSk);
    await event;
  });

  afterEach(function () {
    expect(fetchMock.done()).to.be.true; // All mock RPCs called at least once.
    expectNoUnmatchedCalls(fetchMock);

    // Completely remove the mocking which allows each test
    // to be able to mess with the mocked routes w/o impacting other tests.
    fetchMock.reset();
    // reset the time
    Date.now = regularNow;
    // Remove the stale instance under test.
    if (ignoresSk) {
      document.body.removeChild(ignoresSk);
      ignoresSk = null;
    }
  });

  //===============TESTS START====================================

  describe('html layout', () => {
    it('should make a table with 10 rows in the body', () => {
      const rows = $('table tbody tr', ignoresSk);
      expect(rows).to.have.length(10);
    });

    it('creates links to test the filter', () => {
      const rows = $('table tbody tr', ignoresSk);
      const queryLink = $$('.query a', rows[9]);
      expect(queryLink).to.not.be.null;
      expect(queryLink.href).to.contain('include=true&query=config%3Dglmsaa4%26cpu_or_gpu_value%3DTegraX1%26name%3Drg1024_green_grapes.svg');
      expect(queryLink.textContent).to.equal(`config=glmsaa4\ncpu_or_gpu_value=TegraX1\nname=rg1024_green_grapes.svg`);
    });

    it('has some expired and some not expired rules', () => {
      const rows = $('table tbody tr', ignoresSk);
      const firstRow = rows[0];
      expect(firstRow.className).to.contain('expired');
      let timeBox = $$('.expired', firstRow);
      expect(timeBox.textContent).to.contain('Expired');

      const fourthRow = rows[4];
      expect(fourthRow.className).to.not.contain('expired');
      timeBox = $$('.expired', fourthRow);
      expect(timeBox).to.be.null;
    });
  }); // end describe('html layout')
});