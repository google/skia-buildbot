import './index.js'

import { $, $$ } from 'common-sk/modules/dom'
import { twoPatchSets } from './test_data'

describe('changelist-controls-sk', () => {

  // A reusable HTML element in which we create our element under test.
  let container;

  // calls the test callback with an element under test 'ele'.
  // We can't put the describes inside the whenDefined callback because
  // that doesn't work on Firefox (and possibly other places).
  function createElement(test) {
    return window.customElements.whenDefined('changelist-controls-sk').then(() => {
      container.innerHTML = `<changelist-controls-sk></changelist-controls-sk>`;
      expect(container.firstElementChild).to.not.be.null;
      test(container.firstElementChild);
    });
  }

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
  });

  afterEach(() => {
    document.body.removeChild(container);
  });

  //===============TESTS START====================================

  describe('html layout', () => {
    it('is empty with no data', () => {
      return createElement((ele) => {
        expect(ele.children.length).to.equal(0);
      });
    });

    it('shows the latest patchset by default', () => {
      return createElement((ele) => {
        expect(ele.ps_order).to.equal(0);
        expect(ele.include_master).to.equal(false);

        ele.setSummary(twoPatchSets);
        const psSelector = $$('.inputs select', ele);
        expect(psSelector).to.not.be.null;
        expect(psSelector.value).to.equal('PS 4');

        const includeMasterRadios = $('.inputs radio-sk', ele);
        expect(includeMasterRadios.length).to.equal(2);
        expect(includeMasterRadios[0].hasAttribute('checked')).to.equal(true);

        expect(ele.ps_order).to.equal(4);

        const tryJobs = $('.tryjob-container .tryjob', ele);
        expect(tryJobs).to.not.be.null;
        expect(tryJobs.length).to.equal(4);
        // spot check a tryjob
        expect(tryJobs[0].textContent.trim()).to.equal('android-marshmallow-arm64-rel');
      });
    });

    it('shows other patchsets when ps_order is changed', () => {
      return createElement((ele) => {
        ele.setSummary(twoPatchSets);
        ele.ps_order = 1;
        const psSelector = $$('.inputs select', ele);
        expect(psSelector).to.not.be.null;
        expect(psSelector.value).to.equal('PS 1');

        const tryJobs = $('.tryjob-container .tryjob', ele);
        expect(tryJobs).to.not.be.null;
        expect(tryJobs.length).to.equal(1);
        // spot check a tryjob
        expect(tryJobs[0].textContent.trim()).to.equal('android-nougat-arm64-rel');
      });
    });

    it('flips the radio buttons on include_master', () => {
      return createElement((ele) => {
        ele.setSummary(twoPatchSets);
        const includeMasterRadios = $('.inputs radio-sk', ele);
        expect(includeMasterRadios.length).to.equal(2);
        expect(includeMasterRadios[0].hasAttribute('checked')).to.equal(true);
        expect(includeMasterRadios[1].hasAttribute('checked')).to.equal(false);

        ele.include_master = true;
        expect(includeMasterRadios[0].hasAttribute('checked')).to.equal(false);
        expect(includeMasterRadios[1].hasAttribute('checked')).to.equal(true);
      });
    });
  }); // end describe('html layout')

  describe('events', () => {
    it('generates a cl-control-change event on master toggle', (done) => {
      createElement((ele) => {
        ele.include_master = false;
        ele.setSummary(twoPatchSets);

        ele.addEventListener('cl-control-change', (e) => {
          expect(e.detail.include_master).to.equal(true);
          expect(e.detail.ps_order).to.equal(4);
          done();
        });

        const includeMasterRadios = $('.inputs radio-sk', ele);
        expect(includeMasterRadios.length).to.equal(2);
        includeMasterRadios[1].click();
        expect(ele.include_master).to.equal(true);
        expect(includeMasterRadios[0].hasAttribute('checked')).to.equal(false);
        expect(includeMasterRadios[1].hasAttribute('checked')).to.equal(true);
      });
    });

    it('generates a cl-control-change event on patchset change', (done) => {
      createElement((ele) => {
        ele.ps_order = 0;
        ele.setSummary(twoPatchSets);

        ele.addEventListener('cl-control-change', (e) => {
          expect(e.detail.include_master).to.equal(false);
          expect(e.detail.ps_order).to.equal(1);
          done();
        });

        const psSelector = $$('.inputs select', ele);
        expect(psSelector).to.not.be.null;
        expect(psSelector.value).to.equal('PS 4');

        psSelector.selectedIndex = 0;
        // we have to manually send this because just changing selectedIdx isn't enough.
        // https://stackoverflow.com/a/23612498
        psSelector.dispatchEvent(new Event('input'));
      });
    });
  }); // end describe('events')

});
