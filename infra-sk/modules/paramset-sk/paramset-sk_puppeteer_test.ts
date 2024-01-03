import { expect } from 'chai';
import {
  addEventListenersToPuppeteerPage,
  EventPromiseFactory,
  takeScreenshot,
  TestBed,
  loadCachedTestBed,
} from '../../../puppeteer-tests/util';
import {
  ParamSetSkCheckboxClickEventDetail,
  ParamSetSkClickEventDetail,
  ParamSetSkRemoveClickEventDetail,
} from './paramset-sk';
import { ParamSetSkPO } from './paramset-sk_po';

describe('paramset-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });
  let eventPromise: EventPromiseFactory;

  beforeEach(async () => {
    eventPromise = await addEventListenersToPuppeteerPage(testBed.page, [
      'paramset-key-click',
      'paramset-key-value-click',
      'paramset-value-remove-click',
      'paramset-checkbox-click',
    ]);
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 800, height: 600 });
  });

  describe('screenshots', () => {
    it('has one ParamSet, no titles', async () => {
      const paramSetSk = await testBed.page.$('#one-paramset-no-titles');
      await takeScreenshot(
        paramSetSk!,
        'infra-sk',
        'paramset-sk_one-paramset_no-titles'
      );
    });

    it('has one ParamSet, with titles', async () => {
      const paramSetSk = await testBed.page.$('#one-paramset-with-titles');
      await takeScreenshot(
        paramSetSk!,
        'infra-sk',
        'paramset-sk_one-paramset_with-titles'
      );
    });

    it('has many ParamSets, no titles', async () => {
      const paramSetSk = await testBed.page.$('#many-paramsets-no-titles');
      await takeScreenshot(
        paramSetSk!,
        'infra-sk',
        'paramset-sk_many-paramsets_no-titles'
      );
    });

    it('has many ParamSets, with titles', async () => {
      const paramSetSk = await testBed.page.$('#many-paramsets-with-titles');
      await takeScreenshot(
        paramSetSk!,
        'infra-sk',
        'paramset-sk_many-paramsets_with-titles'
      );
    });

    it('has one ParamSet, with clickable plus and clickable values', async () => {
      const paramSetSk = await testBed.page.$(
        '#clickable-plus-with-clickable-values'
      );
      await takeScreenshot(
        paramSetSk!,
        'infra-sk',
        'paramset-sk_clickable-plus-with-clickable-values'
      );
    });

    it('has one ParamSet, with removable values', async () => {
      const paramSetSk = await testBed.page.$('#removable-values');
      await takeScreenshot(
        paramSetSk!,
        'infra-sk',
        'paramset-sk_removable-values'
      );
    });

    it('has one ParamSet, with checkbox values', async () => {
      const paramSetSk = await testBed.page.$('#checkbox-values');
      await takeScreenshot(
        paramSetSk!,
        'infra-sk',
        'paramset-sk_checkbox-values'
      );
    });
  });

  describe('clicking keys and values', () => {
    let paramSetSkPO: ParamSetSkPO;

    beforeEach(async () => {
      paramSetSkPO = new ParamSetSkPO(
        (await testBed.page.$(
          '#many-paramsets-with-titles-keys-and-values-clickable'
        ))!
      );
    });

    describe('clicking keys', () => {
      it('emits event when "paramset-key-click" clicking a key', async () => {
        const event =
          eventPromise<ParamSetSkClickEventDetail>('paramset-key-click');
        await paramSetSkPO.clickKey('arch');
        const expected: ParamSetSkClickEventDetail = {
          key: 'arch',
          ctrl: false,
        };
        expect(await event).to.deep.equal(expected);
      });

      it('event detail\'s "ctrl" field is set when ctrl key is pressed', async () => {
        await testBed.page.keyboard.down('ControlLeft');
        const event =
          eventPromise<ParamSetSkClickEventDetail>('paramset-key-click');
        await paramSetSkPO.clickKey('arch');
        const expected: ParamSetSkClickEventDetail = {
          key: 'arch',
          ctrl: true,
        };
        expect(await event).to.deep.equal(expected);
      });
    });

    describe('clicking values', () => {
      it('emits event "paramset-key-value-click" when clicking a value', async () => {
        const event = eventPromise<ParamSetSkClickEventDetail>(
          'paramset-key-value-click'
        );
        await paramSetSkPO.clickValue({
          paramSetIndex: 0,
          key: 'arch',
          value: 'x86',
        });
        const expected: ParamSetSkClickEventDetail = {
          key: 'arch',
          value: 'x86',
          ctrl: false,
        };
        expect(await event).to.deep.equal(expected);
      });

      it('event detail\'s "ctrl" field is set when ctrl key is pressed', async () => {
        await testBed.page.keyboard.down('ControlLeft');
        const event = eventPromise<ParamSetSkClickEventDetail>(
          'paramset-key-value-click'
        );
        await paramSetSkPO.clickValue({
          paramSetIndex: 0,
          key: 'arch',
          value: 'x86',
        });
        const expected: ParamSetSkClickEventDetail = {
          key: 'arch',
          value: 'x86',
          ctrl: true,
        };
        expect(await event).to.deep.equal(expected);
      });
    });

    describe('click remove value', () => {
      beforeEach(async () => {
        paramSetSkPO = new ParamSetSkPO(
          (await testBed.page.$('#removable-values'))!
        );
      });
      it('emits the necessary event when remove is clicked.', async () => {
        const event = eventPromise<ParamSetSkRemoveClickEventDetail>(
          'paramset-value-remove-click'
        );
        const key = 'arch';
        const val = 'x86';
        // Let's select a value first
        await paramSetSkPO.clickValue({
          paramSetIndex: 0,
          key: key,
          value: val,
        });
        // Now click on the remove icon next to the selected value
        await paramSetSkPO.removeSelectedValue(key, val);

        const expected: ParamSetSkRemoveClickEventDetail = {
          key: 'arch',
          value: 'x86',
        };
        expect(await event).to.deep.equal(expected);
      });
    });

    describe('click checkbox value', () => {
      beforeEach(async () => {
        paramSetSkPO = new ParamSetSkPO(
          (await testBed.page.$('#checkbox-values'))!
        );
      });
      it('emits the necessary event when checkbox is clicked.', async () => {
        const event = eventPromise<ParamSetSkRemoveClickEventDetail>(
          'paramset-checkbox-click'
        );

        // Let's click a value to unselect the checkbox
        await testBed.page.click('checkbox-sk[label="x86"]');

        const expectedUselected: ParamSetSkCheckboxClickEventDetail = {
          key: 'arch',
          value: 'x86',
          selected: false,
        };
        expect(await event).to.deep.equal(expectedUselected);

        // Now click the same value again to select the checkbox
        const event2 = eventPromise<ParamSetSkRemoveClickEventDetail>(
          'paramset-checkbox-click'
        );
        await testBed.page.click('checkbox-sk[label="x86"]');

        const expectedSelected: ParamSetSkCheckboxClickEventDetail = {
          key: 'arch',
          value: 'x86',
          selected: true,
        };
        expect(await event2).to.deep.equal(expectedSelected);
      });
    });
  });
});
