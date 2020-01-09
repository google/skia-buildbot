import './index.js';

import { $, $$ } from 'common-sk/modules/dom';
import {fakeNow, ignoreRules_10} from './test_data';
import { fetchMock }  from 'fetch-mock';

describe('ignores-page-sk', () => {
    // A reusable HTML element in which we create our element under test.
    const container = document.createElement('div');
    document.body.appendChild(container);
    const regularNow = Date.now;

    beforeEach(function () {
        // Clear out any query params we might have to not mess with our current state.
        setQueryString('');
    });

    beforeEach(function () {
        // These are the default offset/page_size params
        fetchMock.get('/json/ignores?counts=1', JSON.stringify(ignoreRules_10));

        fetchMock.catch(404);
        // set the time to our mocked Now
        Date.now = () => fakeNow;
    });

    afterEach(function () {
        // Completely remove the mocking which allows each test
        // to be able to mess with the mocked routes w/o impacting other tests.
        fetchMock.reset();
        // reset the time
        Date.now = regularNow;
    });

    afterEach(function () {
        container.innerHTML = '';
    });

    // calls the test callback with an element under test 'ele'.
    // We can't put the describes inside the whenDefined callback because
    // that doesn't work on Firefox (and possibly other places).
    function createElement(test) {
        return window.customElements.whenDefined('ignores-page-sk').then(() => {
            container.innerHTML = `<ignores-page-sk></ignores-page-sk>`;
            expect(container.firstElementChild).to.not.be.null;
            test(container.firstElementChild);
        });
    }

    function whenPageLoads(test) {
        // The ignores-page-sk emits an 'end-task' event when each fetch finishes.
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
        };
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
        it('should make a table with 10 rows in the body', (done) => {
            whenPageLoads((ele) => {
                const tbl = $$('table', ele);
                expect(tbl).to.not.be.null;
                const rows = $('tbody tr', tbl);
                expect(rows.length).to.equal(10); // one row per item in ignoreRules_10
                done();
            });
        });

        it('creates links to test the filter', (done) => {
            whenPageLoads((ele) => {
                const rows = $('table tbody tr', ele);
                const firstRow = rows[0];
                const queryLink = $$('.query a', firstRow);
                expect(queryLink).to.not.be.null;
                expect(queryLink.href).to.contain('include=true&query=config%3Dgles%26model%3DiPhone7%26name%3Dglyph_pos_h_s_this_is_a_super_long_test_name_or_key_value');
                expect(queryLink.textContent).to.equal(`config=gles\nmodel=iPhone7\nname=glyph_pos_h_s_this_is_a_super_long_test_name_or_key_value`);
                done();
            });
        });

        it('has some expired and some not expired rules', (done) => {
            whenPageLoads((ele) => {
                const rows = $('table tbody tr', ele);
                const firstRow = rows[0];
                expect(firstRow.classList.contains('expired')).to.be.true;
                let timeBox = $$('.expired', firstRow);
                expect(timeBox.textContent).to.contain('Expired');

                const fourthRow = rows[4];
                expect(fourthRow.classList.contains('expired')).to.be.false;
                timeBox = $$('.expired', fourthRow);
                expect(timeBox).to.be.null;
                done();
            });
        });
    }); // end describe('html layout')

    function setQueryString(q) {
        history.pushState(
            null, '', window.location.origin + window.location.pathname + q);
    }

});