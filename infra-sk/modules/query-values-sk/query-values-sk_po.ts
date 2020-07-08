import { PageObject } from '../page_object/page_object';
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';

/** A page object for the QueryValuesSk component. */
export class QueryValuesSkPO extends PageObject {
  isInvertCheckboxChecked() {
    return this.selectOneDOMNodeThenApplyFn(
      'checkbox-sk#invert', (c) => (c as CheckOrRadio).checked);
  }

  isRegexCheckboxChecked() {
    return this.selectOneDOMNodeThenApplyFn(
      'checkbox-sk#regex', (c) => (c as CheckOrRadio).checked);
  }

  clickInvertCheckbox() {
    return this.selectOnePOEThenApplyFn('checkbox-sk#invert', (el) => el.click());
  }

  clickRegexCheckbox() {
    return this.selectOnePOEThenApplyFn('checkbox-sk#regex', (el) => el.click());
  }

  isInvertCheckboxHidden() {
    return this.selectOnePOEThenApplyFn('checkbox-sk#invert', (el) => el.hasAttribute('hidden'));
  }

  isRegexCheckboxHidden() {
    return this.selectOnePOEThenApplyFn('checkbox-sk#regex', (el) => el.hasAttribute('hidden'));
  }

  async getRegexValue() {
    return this.selectOnePOEThenApplyFn('#regexValue', (input) => input.value);
  }

  async setRegexValue(value: string) {
    return this.selectOnePOEThenApplyFn('#regexValue', (input) => input.enterValue(value));
  }

  async clickOption(option: string) {
    const div =
      await this.selectAllPOEThenFind('div', async (div) => (await div.innerText) === option);
    await div!.click();
  }

  getOptions() {
    return this.selectAllPOEThenMap('div', (div) => div.innerText);
  }

  getSelectedOptions() {
    return this.selectAllPOEThenMap('div[selected]', (div) => div.innerText);
  }

  /** Analogous to the "selected" property getter. */
  async getSelected() {
    if (await this.isRegexCheckboxChecked()) {
      const regex = await this.getRegexValue();
      return ['~' + regex];
    }

    const selectedOptions = await this.getSelectedOptions();
    if (await this.isInvertCheckboxChecked()) {
      return selectedOptions.map((option) => '!' + option);
    }
    return selectedOptions;
  }

  /** Analogous to the "selected" property setter. */
  async setSelected(selected: string[]) {
    // Is it a regex?
    if (selected.some((value) => value.startsWith('~'))) {
      // There can only be one regex.
      if (selected.length > 1) {
        throw new Error('invalid selection: regex found in selection of length > 1');
      }

      // Click the regex checkbox if it isn't checked.
      if (!(await this.isRegexCheckboxChecked())) {
        await this.clickRegexCheckbox();
      }

      // Enter the regex value.
      const regex = selected[0].substring(1); // Remove the tilde at the beginning.
      await this.setRegexValue(regex);
    }

    // Is it an inverted selection?
    if (selected.some((value) => value.startsWith('!'))) {
      // If one item is inverted, all items must be inverted as well.
      if (!selected.every((value) => value.startsWith('!'))) {
        throw new Error('invalid selection: inverted and non-inverted items found');
      }

      // Click the invert checkbox if it isn't checked.
      if (!(await this.isInvertCheckboxChecked())) {
        await this.clickInvertCheckbox();
      }

      selected = selected.map((value) => value.substring(1)); // Remove checks.
    }

    // Set the selection by clicking on the options as needed.
    await this.selectAllPOEThenForEach('div', async (div) => {
      const option = await div.innerText;
      const isSelected = await div.hasAttribute('selected');
      const shouldBeSelected = selected.includes(option);

      if (isSelected !== shouldBeSelected) {
        await div.click();
      }
    });
  }
};
