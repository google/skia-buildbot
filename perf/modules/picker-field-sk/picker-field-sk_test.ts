import './index';
import { expect } from 'chai';
import { PickerFieldSk } from './picker-field-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { CheckOrRadio } from '../../../elements-sk/modules/checkbox-sk/checkbox-sk';

describe('picker-field-sk', () => {
  const newInstance = setUpElementUnderTest<PickerFieldSk>('picker-field-sk');

  let element: PickerFieldSk;
  beforeEach(() => {
    element = newInstance((el: PickerFieldSk) => {
      el.label = 'test-label';
    });
  });

  describe('options setter', () => {
    it('sets options and filters primary options', () => {
      const allOptions = ['option1', 'option.with.period', 'option2', 'another.period'];
      element.options = allOptions;

      expect(element.options).to.deep.equal(allOptions);
      expect(element.primaryOptions).to.deep.equal(['option1', 'option2']);
    });

    it('hides the primary checkbox if there are no primary options', async () => {
      element.index = 1;
      element.options = ['option.with.period', 'another.period'];
      await new Promise((resolve) => setTimeout(resolve, 0));
      const checkbox = element.querySelector<CheckOrRadio>('#select-primary');
      expect(checkbox!.hasAttribute('hidden')).to.equal(true);
    });

    it('shows the primary checkbox if there are primary options', async () => {
      element.index = 1;
      element.options = ['option1', 'option.with.period'];
      await new Promise((resolve) => setTimeout(resolve, 0));
      const checkbox = element.querySelector<CheckOrRadio>('#select-primary');
      expect(checkbox!.hasAttribute('hidden')).to.equal(false);
    });
  });

  describe('selectAll checkbox', () => {
    beforeEach(async () => {
      element.options = ['A', 'B', 'C'];
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    it('selects all items when checked', async () => {
      const selectAllCheckbox = element.querySelector<CheckOrRadio>('#select-all')!;
      selectAllCheckbox.checked = true;
      selectAllCheckbox.dispatchEvent(new Event('change', { bubbles: true }));
      await new Promise((resolve) => setTimeout(resolve, 0));

      expect(element.selectedItems).to.deep.equal(['A', 'B', 'C']);
    });

    it('leaves the first item selected when unchecked', async () => {
      element.selectedItems = ['A', 'B', 'C'];
      await new Promise((resolve) => setTimeout(resolve, 0));

      const selectAllCheckbox = element.querySelector<CheckOrRadio>('#select-all')!;
      selectAllCheckbox.checked = false;
      selectAllCheckbox.dispatchEvent(new Event('change', { bubbles: true }));
      await new Promise((resolve) => setTimeout(resolve, 0));

      expect(element.selectedItems).to.deep.equal(['A']);
    });
  });

  describe('selectPrimary checkbox', () => {
    beforeEach(async () => {
      element.options = ['A', 'B.period', 'C', 'D.period'];
      await new Promise((resolve) => setTimeout(resolve, 0));
      // primaryOptions will be ['A', 'C']
    });

    it('adds primary items to selection when checked', async () => {
      element.selectedItems = ['B.period'];
      await new Promise((resolve) => setTimeout(resolve, 0));

      const selectPrimaryCheckbox = element.querySelector<CheckOrRadio>('#select-primary')!;
      selectPrimaryCheckbox.checked = true;
      selectPrimaryCheckbox.dispatchEvent(new Event('change', { bubbles: true }));
      await new Promise((resolve) => setTimeout(resolve, 0));

      // sort for consistent comparison
      expect(element.selectedItems.sort()).to.deep.equal(['A', 'B.period', 'C'].sort());
    });

    it('leaves only the first item selected when unchecked', async () => {
      element.selectedItems = ['A', 'B.period', 'C'];
      await new Promise((resolve) => setTimeout(resolve, 0));

      const selectPrimaryCheckbox = element.querySelector<CheckOrRadio>('#select-primary')!;
      selectPrimaryCheckbox.checked = false;
      selectPrimaryCheckbox.dispatchEvent(new Event('change', { bubbles: true }));
      await new Promise((resolve) => setTimeout(resolve, 0));

      expect(element.selectedItems).to.deep.equal(['A']);
    });

    it('selects only primary items when all items were previously selected', async () => {
      element.selectedItems = ['A', 'B.period', 'C', 'D.period'];
      await new Promise((resolve) => setTimeout(resolve, 0));

      const selectPrimaryCheckbox = element.querySelector<CheckOrRadio>('#select-primary')!;
      selectPrimaryCheckbox.checked = true;
      selectPrimaryCheckbox.dispatchEvent(new Event('change', { bubbles: true }));
      await new Promise((resolve) => setTimeout(resolve, 0));

      expect(element.selectedItems.sort()).to.deep.equal(['A', 'C'].sort());
    });
  });

  describe('split checkbox', () => {
    beforeEach(async () => {
      // Must set index > 0 and multiple selection for split to be visible
      element.index = 1;
      element.options = ['A', 'B'];
      element.selectedItems = ['A', 'B'];
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    it('is visible and enabled by default when criteria met', () => {
      const splitCheckbox = element.querySelector<CheckOrRadio>('#split-by');
      expect(splitCheckbox!.hasAttribute('hidden')).to.be.false;
      expect(splitCheckbox!.hasAttribute('disabled')).to.be.false;
    });

    it('is disabled but visible when disableSplit() is called', async () => {
      element.disableSplit();
      await new Promise((resolve) => setTimeout(resolve, 0));
      const splitCheckbox = element.querySelector<CheckOrRadio>('#split-by');
      expect(splitCheckbox!.hasAttribute('hidden')).to.be.false;
      expect(splitCheckbox!.hasAttribute('disabled')).to.be.true;
    });

    it('is re-enabled when enableSplit() is called', async () => {
      element.disableSplit();
      await new Promise((resolve) => setTimeout(resolve, 0));
      element.enableSplit();
      await new Promise((resolve) => setTimeout(resolve, 0));
      const splitCheckbox = element.querySelector<CheckOrRadio>('#split-by');
      expect(splitCheckbox!.hasAttribute('disabled')).to.be.false;
    });
  });

  describe('vaadin-multi-select-combo-box', () => {
    it('is visible when rendered', async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
      const comboBox = element.querySelector('vaadin-multi-select-combo-box');
      expect(comboBox).to.not.equal(null);
      expect(comboBox!.offsetWidth).to.be.greaterThan(0);
      expect(comboBox!.offsetHeight).to.be.greaterThan(0);
    });
  });
});
