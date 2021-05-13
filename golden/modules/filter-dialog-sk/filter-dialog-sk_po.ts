import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { TraceFilterSkPO } from '../trace-filter-sk/trace-filter-sk_po';
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';
import { Filters } from './filter-dialog-sk';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

/**
 * A page object for the numeric parameter input fields in FilterDialogSk.
 *
 * This type of field is composed of an <input type=range> and an <input type=number> that reflect
 * each other's values (when one changes, the other changes as well).
 */
export class NumericParamPO extends PageObject {
  private get rangeInput(): PageObjectElement {
    return this.bySelector('input[type=range]');
  }

  private get numberInput(): PageObjectElement {
    return this.bySelector('input[type=number]')
  }

  async focusRangeInput() { await this.rangeInput.focus(); }

  async focusNumberInput() { await this.numberInput.focus(); }

  async getRangeInputValue() { return parseInt(await this.rangeInput.value); }

  async setRangeInputValue(value: number) { await this.rangeInput.enterValue(value.toString()); }

  async geNumberInputValue() { return parseInt(await this.numberInput.value); }

  async setNumberInputValue(value: number) { await this.numberInput.enterValue(value.toString()); }
}

/** A page object for the FilterDialogSk component. */
export class FilterDialogSkPO extends PageObject {
  get traceFilterSkPO(): TraceFilterSkPO {
    return this.poBySelector('trace-filter-sk', TraceFilterSkPO);
  }

  get minRGBADeltaPO(): NumericParamPO {
    return this.poBySelector('#min-rgba-delta-numeric-param', NumericParamPO);
  }

  get maxRGBADeltaPO(): NumericParamPO {
    return this.poBySelector('#max-rgba-delta-numeric-param', NumericParamPO);
  }

  private get mustHaveReferenceImageCheckBox(): PageObjectElement {
    return this.bySelector('#must-have-reference-image');
  }

  private get sortOrderDropDown(): PageObjectElement {
    return this.bySelector('select#sort-order');
  }

  private get filterDialog(): PageObjectElement {
    return this.bySelector('dialog.filter-dialog');
  }

  private get filterDialogFilterBtn(): PageObjectElement {
    return this.bySelector('dialog.filter-dialog > .buttons > .filter');
  }

  private get filterDialogCancelBtn(): PageObjectElement {
    return this.bySelector('dialog.filter-dialog > .buttons > .cancel');
  }

  async isDialogOpen() {
    return this.filterDialog.applyFnToDOMNode((dialog) => (dialog as HTMLDialogElement).open);
  }

  async getMinRGBADelta() { return this.minRGBADeltaPO.getRangeInputValue(); }

  async setMinRGBADelta(value: number) { await this.minRGBADeltaPO.setRangeInputValue(value); }

  async getMaxRGBADelta() { return this.maxRGBADeltaPO.getRangeInputValue(); }

  async setMaxRGBADelta(value: number) { await this.maxRGBADeltaPO.setRangeInputValue(value); }

  async getSortOrder() { return await this.sortOrderDropDown.value as 'ascending' | 'descending'; }

  async setSortOrder(value: 'ascending' | 'descending') {
    await this.sortOrderDropDown.enterValue(value);
  }

  async isReferenceImageCheckboxChecked() {
    return this.mustHaveReferenceImageCheckBox
        .applyFnToDOMNode((c) => (c as CheckOrRadio).checked);
  }

  async clickReferenceImageCheckbox() { await this.mustHaveReferenceImageCheckBox.click(); }

  async clickFilterBtn() { await this.filterDialogFilterBtn.click(); }

  async clickCancelBtn() { await this.filterDialogCancelBtn.click(); }

  /** Gets the selected filters. */
  async getSelectedFilters() {
    const filters: Filters = {
      diffConfig: await this.traceFilterSkPO.getSelection(),
      minRGBADelta: await this.getMinRGBADelta(),
      maxRGBADelta: await this.getMaxRGBADelta(),
      sortOrder: await this.getSortOrder(),
      mustHaveReferenceImage: await this.isReferenceImageCheckboxChecked()
    }
    return filters;
  }

  /** Sets the selected filters. */
  async setSelectedFilters(filters: Filters) {
    await this.traceFilterSkPO.clickEditBtn();
    await this.traceFilterSkPO.setQueryDialogSkSelection(filters.diffConfig);
    await this.traceFilterSkPO.clickQueryDialogSkShowMatchesBtn();

    await this.setMinRGBADelta(filters.minRGBADelta);
    await this.setMaxRGBADelta(filters.maxRGBADelta);
    await this.setSortOrder(filters.sortOrder);

    if (filters.mustHaveReferenceImage !== (await this.isReferenceImageCheckboxChecked())) {
      await this.clickReferenceImageCheckbox();
    }
  }
}
