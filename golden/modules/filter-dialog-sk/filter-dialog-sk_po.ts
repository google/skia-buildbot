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
  private get rangeInput(): Promise<PageObjectElement> {
    return this.selectOnePOE('input[type=range]');
  }

  private get numberInput(): Promise<PageObjectElement> {
    return this.selectOnePOE('input[type=number]')
  }

  async focusRangeInput() { await (await this.rangeInput).focus(); }

  async focusNumberInput() { await (await this.numberInput).focus(); }

  async getRangeInputValue() { return parseInt(await (await this.rangeInput).value); }

  async setRangeInputValue(value: number) {
    await (await this.rangeInput).enterValue(value.toString());
  }

  async geNumberInputValue() { return parseInt(await (await this.numberInput).value); }

  async setNumberInputValue(value: number) {
    await (await this.numberInput).enterValue(value.toString());
  }
}

/** A page object for the FilterDialogSk component. */
export class FilterDialogSkPO extends PageObject {
  get traceFilterSkPO(): Promise<TraceFilterSkPO> {
    return this.poBySelector('trace-filter-sk', TraceFilterSkPO);
  }

  get minRGBADeltaPO(): Promise<NumericParamPO> {
    return this.poBySelector('#min-rgba-delta-numeric-param', NumericParamPO);
  }

  get maxRGBADeltaPO(): Promise<NumericParamPO> {
    return this.poBySelector('#max-rgba-delta-numeric-param', NumericParamPO);
  }

  private get mustHaveReferenceImageCheckBox(): Promise<PageObjectElement> {
    return this.selectOnePOE('#must-have-reference-image');
  }

  private get sortOrderDropDown(): Promise<PageObjectElement> {
    return this.selectOnePOE('select#sort-order');
  }

  private get filterDialog(): Promise<PageObjectElement> {
    return this.selectOnePOE('dialog.filter-dialog');
  }

  private get filterDialogFilterBtn(): Promise<PageObjectElement> {
    return this.selectOnePOE('dialog.filter-dialog > .buttons > .filter');
  }

  private get filterDialogCancelBtn(): Promise<PageObjectElement> {
    return this.selectOnePOE('dialog.filter-dialog > .buttons > .cancel');
  }

  async isDialogOpen() {
    return (await this.filterDialog)
        .applyFnToDOMNode((dialog) => (dialog as HTMLDialogElement).open);
  }

  async getMinRGBADelta() { return (await this.minRGBADeltaPO).getRangeInputValue(); }

  async setMinRGBADelta(value: number) {
    return (await this.minRGBADeltaPO).setRangeInputValue(value);
  }

  async getMaxRGBADelta() { return (await this.maxRGBADeltaPO).getRangeInputValue(); }

  async setMaxRGBADelta(value: number) {
    return (await this.maxRGBADeltaPO).setRangeInputValue(value);
  }

  async getSortOrder() {
    return await (await this.sortOrderDropDown).value as 'ascending' | 'descending';
  }

  async setSortOrder(value: 'ascending' | 'descending') {
    await (await this.sortOrderDropDown).enterValue(value);
  }

  async isReferenceImageCheckboxChecked() {
    return (await this.mustHaveReferenceImageCheckBox)
        .applyFnToDOMNode((c) => (c as CheckOrRadio).checked);
  }

  async clickReferenceImageCheckbox() {
    return (await this.mustHaveReferenceImageCheckBox).click();
  }

  async clickFilterBtn() { await (await this.filterDialogFilterBtn).click(); }

  async clickCancelBtn() { await (await this.filterDialogCancelBtn).click(); }

  /** Gets the selected filters. */
  async getSelectedFilters() {
    const filters: Filters = {
      diffConfig: await (await this.traceFilterSkPO).getSelection(),
      minRGBADelta: await this.getMinRGBADelta(),
      maxRGBADelta: await this.getMaxRGBADelta(),
      sortOrder: await this.getSortOrder(),
      mustHaveReferenceImage: await this.isReferenceImageCheckboxChecked()
    }
    return filters;
  }

  /** Sets the selected filters. */
  async setSelectedFilters(filters: Filters) {
    const traceFilterSkPO = await this.traceFilterSkPO;
    await traceFilterSkPO.clickEditBtn();
    await traceFilterSkPO.setQueryDialogSkSelection(filters.diffConfig);
    await traceFilterSkPO.clickQueryDialogSkShowMatchesBtn();

    await this.setMinRGBADelta(filters.minRGBADelta);
    await this.setMaxRGBADelta(filters.maxRGBADelta);
    await this.setSortOrder(filters.sortOrder);

    if (filters.mustHaveReferenceImage !== (await this.isReferenceImageCheckboxChecked())) {
      await this.clickReferenceImageCheckbox();
    }
  }
}
