import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';
import { PageObject } from '../page_object/page_object';
import { PageObjectElement, PageObjectElementList } from '../page_object/page_object_element';
import { asyncForEach } from '../async';

/** A page object for the QueryValuesSk component. */
export class QueryValuesSkPO extends PageObject {
  private get invertCheckBox(): PageObjectElement {
    return this.bySelector('checkbox-sk#invert');
  }

  private get regexCheckBox(): PageObjectElement {
    return this.bySelector('checkbox-sk#regex');
  }

  private get regexInput(): PageObjectElement {
    return this.bySelector('#regexValue');
  }

  private get options(): PageObjectElementList {
    return this.bySelectorAll('multi-select-sk#values div');
  }

  private get selectedOptions(): PageObjectElementList {
    return this.bySelectorAll('multi-select-sk#values div[selected]');
  }

  async isInvertCheckboxChecked() {
    return (await this.invertCheckBox)
      .applyFnToDOMNode((c: HTMLElement) => (c as CheckOrRadio).checked);
  }

  async isRegexCheckboxChecked() {
    return (await this.regexCheckBox)
      .applyFnToDOMNode((c: HTMLElement) => (c as CheckOrRadio).checked);
  }

  async clickInvertCheckbox() { await (await this.invertCheckBox).click(); }

  async clickRegexCheckbox() { await (await this.regexCheckBox).click(); }

  async isInvertCheckboxHidden() { return (await this.invertCheckBox).hasAttribute('hidden'); }

  async isRegexCheckboxHidden() { return (await this.regexCheckBox).hasAttribute('hidden'); }

  async getRegexValue() { return (await this.regexInput).value; }

  async setRegexValue(value: string) { await (await this.regexInput).enterValue(value); }

  async clickOption(option: string) {
    const optionDiv = await this.options.find((div) => div.isInnerTextEqualTo(option));
    await optionDiv?.click();
  }

  getOptions() { return this.options.map((option) => option.innerText); }

  getSelectedOptions() { return this.selectedOptions.map((option) => option.innerText); }

  /** Analogous to the "selected" property getter. */
  async getSelected() {
    if (await this.isRegexCheckboxChecked()) {
      const regex = await this.getRegexValue();
      return [`~${regex}`];
    }

    const selectedOptions = await this.getSelectedOptions();
    if (await this.isInvertCheckboxChecked()) {
      return selectedOptions.map((option) => `!${option}`);
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
    const currentlySelectedOptions = await this.getSelectedOptions();
    await asyncForEach(this.getOptions(), async (option) => {
      const isSelected = currentlySelectedOptions.includes(option);
      const shouldBeSelected = selected.includes(option);
      if (isSelected !== shouldBeSelected) {
        await this.clickOption(option);
      }
    });
  }
}
