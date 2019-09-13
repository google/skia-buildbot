import './index.js'

import { changelistSummaries_5 } from './test_data'
import { $, $$ } from 'common-sk/modules/dom'

describe('changelists-page-sk', () => {

  const { fetchMock, MATCHED, UNMATCHED } = require('fetch-mock');

  // A reusable HTML element in which we create our element under test.
  const container = document.createElement('div');
  document.body.appendChild(container);

  beforeEach(function() {
    fetchMock.get('/json/changelists', JSON.stringify(changelistSummaries_5));
    // Everything else
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

  // calls the test callback with one element 'ele', a created <swarming-app>.
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
        const rows = $('tbody tr');
        expect(rows.length).to.equal(5); // one row per item in changelistSummaries_5
        done()
      });
    });
  }); // end describe('html layout')

});