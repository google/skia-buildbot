import { CheckOrRadio } from '../../../elements-sk/modules/checkbox-sk/checkbox-sk';
import { PageObject } from '../page_object/page_object';
import { PageObjectElement, PageObjectElementList } from '../page_object/page_object_element';
import { asyncForEach } from '../async';
import { QueryValuesSk } from './query-values-sk';

/** A page object for the QueryValuesSk component. */
export class QueryValuesSkPO extends PageObject {
  private uniqueIdPromise: Promise<string> = this.element.applyFnToDOMNode(
    (el) => (el as QueryValuesSk).uniqueId
  );

  private async getInvertCheckBox(): Promise<PageObjectElement> {
    return this.bySelector(`checkbox-sk#invert-${await this.uniqueIdPromise}`);
  }

  private async getRegexCheckBox(): Promise<PageObjectElement> {
    return this.bySelector(`checkbox-sk#regex-${await this.uniqueIdPromise}`);
  }

  private async getRegexInput(): Promise<PageObjectElement> {
    return this.bySelector(`#regexValue-${await this.uniqueIdPromise}`);
  }

  private async getFilterInput(): Promise<PageObjectElement> {
    return this.bySelector(`#filter-${await this.uniqueIdPromise}`);
  }

  private async getOptionsList(): Promise<PageObjectElementList> {
    return this.bySelectorAll(`multi-select-sk#values-${await this.uniqueIdPromise} div`);
  }

  private async getSelectedOptionsList(): Promise<PageObjectElementList> {
    return this.bySelectorAll(`multi-select-sk#values-${await this.uniqueIdPromise} div[selected]`);
  }

  private get clearFiltersBtn(): PageObjectElement {
    return this.bySelector('button.clear_filters');
  }

  async isInvertCheckboxChecked() {
    return (await this.getInvertCheckBox()).applyFnToDOMNode(
      (c: Element) => (c as CheckOrRadio).checked
    );
  }

  async isRegexCheckboxChecked() {
    return (await this.getRegexCheckBox()).applyFnToDOMNode(
      (c: Element) => (c as CheckOrRadio).checked
    );
  }

  async clickInvertCheckbox() {
    await (await this.getInvertCheckBox()).click();
  }

  async clickRegexCheckbox() {
    await (await this.getRegexCheckBox()).click();
  }

  async isInvertCheckboxHidden() {
    return (await this.getInvertCheckBox()).hasAttribute('hidden');
  }

  async isRegexCheckboxHidden() {
    return (await this.getRegexCheckBox()).hasAttribute('hidden');
  }

  async getRegexValue() {
    return (await this.getRegexInput()).value;
  }

  async setRegexValue(value: string) {
    await (await this.getRegexInput()).enterValue(value);
  }

  async getFilterInputValue() {
    return (await this.getFilterInput()).value;
  }

  async setFilterInputValue(value: string) {
    await (await this.getFilterInput()).enterValue(value);
  }

  async clickOption(option: string) {
    const optionDiv = await (
      await this.getOptionsList()
    ).find((div) => div.isInnerTextEqualTo(option));
    await optionDiv?.click();
  }

  async clickClearFilter() {
    await this.clearFiltersBtn.click();
  }

  async getOptions() {
    return (await this.getOptionsList()).map((option) => option.innerText);
  }

  async getSelectedOptions() {
    return (await this.getSelectedOptionsList()).map((option) => option.innerText);
  }

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
      return; // A regex cannot be combined with other selections.
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

      selected = selected.map((value) => value.substring(1)); // Remove "!" prefixes.
    }

    // Set the selection by clicking on the options as needed.
    const currentlySelectedOptions = await this.getSelectedOptions();
    const allOptions = await this.getOptions();
    await asyncForEach(allOptions, async (option) => {
      const isSelected = currentlySelectedOptions.includes(option);
      const shouldBeSelected = selected.includes(option);
      if (isSelected !== shouldBeSelected) {
        await this.clickOption(option);
      }
    });
  }
}
