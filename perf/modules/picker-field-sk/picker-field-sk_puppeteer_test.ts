import { expect } from 'chai';
import { loadCachedTestBed, TestBed } from '../../../puppeteer-tests/util';
import { PickerFieldSkPO } from './picker-field-sk_po';
import { ElementHandle } from 'puppeteer';
import { assert } from 'chai';

describe('picker-field-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
  });

  it('should render the demo page', async () => {
    expect(await testBed.page.$$('picker-field-sk')).to.have.length(2);
  });

  describe('with default values', () => {
    it('focuses the first box', async () => {
      await testBed.page.click('#demo-focus1');
      const activeElement = await testBed.page.evaluate(() => document.activeElement?.tagName);
      expect(activeElement).to.equal('INPUT');
    });
  });

  describe('with focus and fill', () => {
    let focusAndFillPicker: ElementHandle;
    let focusAndFillPickerPO: PickerFieldSkPO;

    beforeEach(async () => {
      focusAndFillPicker = (await testBed.page.$('#focus-and-fill'))!;
      focusAndFillPickerPO = new PickerFieldSkPO(focusAndFillPicker);
    });

    it('focuses the second box', async () => {
      await testBed.page.click('#demo-focus2');
      await testBed.page.evaluate(() => document.activeElement?.id);
      // The vaadin-multi-select-combo-box is inside the picker-field-sk which has the id.
      // The focus is on the vaadin component, so we check the parent.
      const parentId = await testBed.page.evaluate(
        (el) => el?.parentElement?.parentElement?.parentElement?.id,
        await testBed.page.evaluateHandle(() => document.activeElement)
      );
      expect(parentId).to.equal('focus-and-fill');
    });

    it('autofills the second box', async () => {
      await testBed.page.click('#demo-fill');
      const selectedItems = await focusAndFillPickerPO.getSelectedItems();
      expect(selectedItems).to.deep.equal(['V8', 'speedometer3']);
    });

    it('opens the overlay of the second box', async () => {
      await testBed.page.click('#demo-open');
      const isOpened = await focusAndFillPickerPO.comboBox.hasAttribute('opened');
      assert.isTrue(isOpened);
    });

    it('disables the second box', async () => {
      await testBed.page.click('#demo-disable');
      const isDisabled = await focusAndFillPickerPO.isDisabled();
      assert.isTrue(isDisabled);
    });

    it('enables the second box', async () => {
      // First disable it.
      await testBed.page.click('#demo-disable');
      let isDisabled = await focusAndFillPickerPO.isDisabled();
      assert.isTrue(isDisabled);

      // Then enable it.
      await testBed.page.click('#demo-enable');
      isDisabled = await focusAndFillPickerPO.isDisabled();
      assert.isFalse(isDisabled);
    });
  });
});
