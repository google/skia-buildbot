import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { twoPatchSets } from './test_data';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('changelist-controls-sk', () => {
  const newInstance = setUpElementUnderTest('changelist-controls-sk');

  let changelistControlsSk;
  beforeEach(() => changelistControlsSk = newInstance());

  describe('html layout', () => {
    it('is empty with no data', () => {
      expect(changelistControlsSk.children.length).to.equal(0);
    });

    it('shows the latest patchset by default', () => {
      expect(changelistControlsSk.ps_order).to.equal(0);
      expect(changelistControlsSk.include_master).to.equal(false);

      changelistControlsSk.setSummary(twoPatchSets);
      const psSelector = $$('.inputs select', changelistControlsSk);
      expect(psSelector).to.not.be.null;
      expect(psSelector.value).to.equal('PS 4');

      const includeMasterRadios = $('.inputs radio-sk', changelistControlsSk);
      expect(includeMasterRadios.length).to.equal(2);
      expect(includeMasterRadios[0].hasAttribute('checked')).to.equal(true);

      expect(changelistControlsSk.ps_order).to.equal(4);

      const tryJobs = $('.tryjob-container .tryjob', changelistControlsSk);
      expect(tryJobs).to.not.be.null;
      expect(tryJobs.length).to.equal(4);
      // spot check a tryjob
      expect(tryJobs[0].textContent.trim()).to.equal('android-marshmallow-arm64-rel');
    });

    it('shows other patchsets when ps_order is changed', () => {
      changelistControlsSk.setSummary(twoPatchSets);
      changelistControlsSk.ps_order = 1;
      const psSelector = $$('.inputs select', changelistControlsSk);
      expect(psSelector).to.not.be.null;
      expect(psSelector.value).to.equal('PS 1');

      const tryJobs = $('.tryjob-container .tryjob', changelistControlsSk);
      expect(tryJobs).to.not.be.null;
      expect(tryJobs.length).to.equal(1);
      // spot check a tryjob
      expect(tryJobs[0].textContent.trim()).to.equal('android-nougat-arm64-rel');
    });

    it('flips the radio buttons on include_master', () => {
      changelistControlsSk.setSummary(twoPatchSets);
      const includeMasterRadios = $('.inputs radio-sk', changelistControlsSk);
      expect(includeMasterRadios.length).to.equal(2);
      expect(includeMasterRadios[0].hasAttribute('checked')).to.equal(true);
      expect(includeMasterRadios[1].hasAttribute('checked')).to.equal(false);

      changelistControlsSk.include_master = true;
      expect(includeMasterRadios[0].hasAttribute('checked')).to.equal(false);
      expect(includeMasterRadios[1].hasAttribute('checked')).to.equal(true);
    });
  }); // end describe('html layout')

  describe('events', () => {
    it('generates a cl-control-change event on master toggle', (done) => {
      changelistControlsSk.include_master = false;
      changelistControlsSk.setSummary(twoPatchSets);

      changelistControlsSk.addEventListener('cl-control-change', (e) => {
        expect(e.detail.include_master).to.equal(true);
        expect(e.detail.ps_order).to.equal(4);
        done();
      });

      const includeMasterRadios = $('.inputs radio-sk', changelistControlsSk);
      expect(includeMasterRadios.length).to.equal(2);
      includeMasterRadios[1].click();
      expect(changelistControlsSk.include_master).to.equal(true);
      expect(includeMasterRadios[0].hasAttribute('checked')).to.equal(false);
      expect(includeMasterRadios[1].hasAttribute('checked')).to.equal(true);
    });

    it('generates a cl-control-change event on patchset change', (done) => {
      changelistControlsSk.ps_order = 0;
      changelistControlsSk.setSummary(twoPatchSets);

      changelistControlsSk.addEventListener('cl-control-change', (e) => {
        expect(e.detail.include_master).to.equal(false);
        expect(e.detail.ps_order).to.equal(1);
        done();
      });

      const psSelector = $$('.inputs select', changelistControlsSk);
      expect(psSelector).to.not.be.null;
      expect(psSelector.value).to.equal('PS 4');

      psSelector.selectedIndex = 0;
      // we have to manually send this because just changing selectedIdx isn't enough.
      // https://stackoverflow.com/a/23612498
      psSelector.dispatchEvent(new Event('input'));
    });
  }); // end describe('events')
});
