import './index';

import { expect } from 'chai';
import { ChangelistControlsSk, ChangelistControlsSkChangeEventDetail } from './changelist-controls-sk';
import { ChangelistControlsSkPO } from './changelist-controls-sk_po';
import { twoPatchsets } from './test_data';
import { setUpElementUnderTest, eventPromise } from '../../../infra-sk/modules/test_util';

describe('changelist-controls-sk', () => {
  const newInstance = setUpElementUnderTest<ChangelistControlsSk>('changelist-controls-sk');

  let changelistControlsSk: ChangelistControlsSk;
  let changelistControlsSkPO: ChangelistControlsSkPO;

  beforeEach(() => {
    changelistControlsSk = newInstance();
    changelistControlsSkPO = new ChangelistControlsSkPO(changelistControlsSk);
  });

  describe('html layout', () => {
    it('is empty with no data', async () => {
      expect(await changelistControlsSkPO.isVisible()).to.be.false;
    });

    it('shows the latest patchset by default', async () => {
      expect(changelistControlsSk.ps_order).to.equal(0);
      expect(changelistControlsSk.include_master).to.equal(false);

      changelistControlsSk.summary = twoPatchsets;
      expect(changelistControlsSk.ps_order).to.equal(4);

      expect(await changelistControlsSkPO.getPatchset()).to.equal('PS 4');
      expect(await changelistControlsSkPO.isExcludeResultsFromPrimaryRadioChecked()).to.be.true;
      expect(await changelistControlsSkPO.isShowAllResultsRadioChecked()).to.be.false;
      expect(await changelistControlsSkPO.getTryJobs()).to.deep.equal([
        'android-marshmallow-arm64-rel',
        'linux-rel',
        'mac-rel',
        'win10_chromium_x64_rel_ng',
      ]);
    });

    it('shows other patchsets when ps_order is changed', async () => {
      changelistControlsSk.summary = twoPatchsets;
      changelistControlsSk.ps_order = 1;

      expect(await changelistControlsSkPO.getPatchset()).to.equal('PS 1');
      expect(await changelistControlsSkPO.getTryJobs()).to.deep.equal(['android-nougat-arm64-rel']);
    });

    it('flips the radio buttons on include_master', async () => {
      changelistControlsSk.summary = twoPatchsets;
      expect(await changelistControlsSkPO.isExcludeResultsFromPrimaryRadioChecked()).to.be.true;
      expect(await changelistControlsSkPO.isShowAllResultsRadioChecked()).to.be.false;

      changelistControlsSk.include_master = true;
      expect(await changelistControlsSkPO.isExcludeResultsFromPrimaryRadioChecked()).to.be.false;
      expect(await changelistControlsSkPO.isShowAllResultsRadioChecked()).to.be.true;
    });
  }); // end describe('html layout')

  describe('events', () => {
    it('generates a cl-control-change event on "include results from primary" toggle', async () => {
      changelistControlsSk.include_master = false;
      changelistControlsSk.ps_order = 4;
      changelistControlsSk.summary = twoPatchsets;

      expect(await changelistControlsSkPO.isExcludeResultsFromPrimaryRadioChecked()).to.be.true;
      expect(await changelistControlsSkPO.isShowAllResultsRadioChecked()).to.be.false;

      const event = eventPromise<CustomEvent<ChangelistControlsSkChangeEventDetail>>('cl-control-change');
      await changelistControlsSkPO.clickShowAllResultsRadio();
      const eventDetail = (await event).detail;

      expect(eventDetail.include_master).to.be.true;
      expect(eventDetail.ps_order).to.equal(4);

      expect(changelistControlsSk.include_master).to.equal(true);
      expect(await changelistControlsSkPO.isExcludeResultsFromPrimaryRadioChecked()).to.be.false;
      expect(await changelistControlsSkPO.isShowAllResultsRadioChecked()).to.be.true;
    });

    it('generates a cl-control-change event on patchset change', async () => {
      changelistControlsSk.ps_order = 0; // Use the latest patchset, i.e. 'PS 4'.
      changelistControlsSk.summary = twoPatchsets;

      expect(await changelistControlsSkPO.getPatchset()).to.equal('PS 4');

      const event = eventPromise<CustomEvent<ChangelistControlsSkChangeEventDetail>>('cl-control-change');
      await changelistControlsSkPO.setPatchset('PS 1');
      const eventDetail = (await event).detail;

      expect(eventDetail.include_master).to.equal(false);
      expect(eventDetail.ps_order).to.equal(1);

      expect(changelistControlsSk.ps_order).to.equal(1);
      expect(await changelistControlsSkPO.getPatchset()).to.equal('PS 1');
    });
  }); // end describe('events')
});
