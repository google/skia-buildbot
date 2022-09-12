import { expect } from 'chai';
import {
  addEventListenersToPuppeteerPage,
  EventPromiseFactory,
  takeScreenshot,
  TestBed,
  loadCachedTestBed,
} from '../../../puppeteer-tests/util';
import { ParamSetSkClickEventDetail } from './paramset-sk';
import { ParamSetSkPO } from './paramset-sk_po';

describe('paramset-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });
  let eventPromise: EventPromiseFactory;

  beforeEach(async () => {
    eventPromise = await addEventListenersToPuppeteerPage(
      testBed.page, ['paramset-key-click', 'paramset-key-value-click'],
    );
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 800, height: 600 });
  });

  describe('screenshots', () => {
    it('has one ParamSet, no titles', async () => {
      const paramSetSk = await testBed.page.$('#one-paramset-no-titles');
      await takeScreenshot(paramSetSk!, 'infra-sk', 'paramset-sk_one-paramset_no-titles');
    });

    it('has one ParamSet, with titles', async () => {
      const paramSetSk = await testBed.page.$('#one-paramset-with-titles');
      await takeScreenshot(paramSetSk!, 'infra-sk', 'paramset-sk_one-paramset_with-titles');
    });

    it('has many ParamSets, no titles', async () => {
      const paramSetSk = await testBed.page.$('#many-paramsets-no-titles');
      await takeScreenshot(paramSetSk!, 'infra-sk', 'paramset-sk_many-paramsets_no-titles');
    });

    it('has many ParamSets, with titles', async () => {
      const paramSetSk = await testBed.page.$('#many-paramsets-with-titles');
      await takeScreenshot(paramSetSk!, 'infra-sk', 'paramset-sk_many-paramsets_with-titles');
    });

    it('has one ParamSet, with clickable plus and clickable values', async () => {
      const paramSetSk = await testBed.page.$('#clickable-plus-with-clickable-values');
      await takeScreenshot(paramSetSk!, 'infra-sk', 'paramset-sk_clickable-plus-with-clickable-values');
    });
  });

  describe('clicking keys and values', () => {
    let paramSetSkPO: ParamSetSkPO;

    beforeEach(async () => {
      paramSetSkPO = new ParamSetSkPO(
          (await testBed.page.$('#many-paramsets-with-titles-keys-and-values-clickable'))!,
      );
    });

    describe('clicking keys', () => {
      it('emits event when "paramset-key-click" clicking a key', async () => {
        const event = eventPromise<ParamSetSkClickEventDetail>('paramset-key-click');
        await paramSetSkPO.clickKey('arch');
        const expected: ParamSetSkClickEventDetail = { key: 'arch', ctrl: false };
        expect(await event).to.deep.equal(expected);
      });

      it('event detail\'s "ctrl" field is set when ctrl key is pressed', async () => {
        await testBed.page.keyboard.down('ControlLeft');
        const event = eventPromise<ParamSetSkClickEventDetail>('paramset-key-click');
        await paramSetSkPO.clickKey('arch');
        const expected: ParamSetSkClickEventDetail = { key: 'arch', ctrl: true };
        expect(await event).to.deep.equal(expected);
      });
    });

    describe('clicking values', () => {
      it('emits event "paramset-key-value-click" when clicking a value', async () => {
        const event = eventPromise<ParamSetSkClickEventDetail>('paramset-key-value-click');
        await paramSetSkPO.clickValue({ paramSetIndex: 0, key: 'arch', value: 'x86' });
        const expected: ParamSetSkClickEventDetail = { key: 'arch', value: 'x86', ctrl: false };
        expect(await event).to.deep.equal(expected);
      });

      it('event detail\'s "ctrl" field is set when ctrl key is pressed', async () => {
        await testBed.page.keyboard.down('ControlLeft');
        const event = eventPromise<ParamSetSkClickEventDetail>('paramset-key-value-click');
        await paramSetSkPO.clickValue({ paramSetIndex: 0, key: 'arch', value: 'x86' });
        const expected: ParamSetSkClickEventDetail = { key: 'arch', value: 'x86', ctrl: true };
        expect(await event).to.deep.equal(expected);
      });
    });
  });
});
