import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { TraceFilterSkPO } from '../trace-filter-sk/trace-filter-sk_po';
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';
import { Filters } from './filter-dialog-sk';

/** A page object for the FilterDialogSk component. */
export class FilterDialogSkPO extends PageObject {
  getTraceFilterSkPO() {
    return this.selectOnePOEThenApplyFn(
      'trace-filter-sk', async (el) => new TraceFilterSkPO(el));
  }

  getMinRGBADeltaPO() {
    return this.selectOnePOEThenApplyFn(
      '#min-rgba-delta-numeric-param', async (el) => new NumericParamPO(el));
  }

  getMaxRGBADeltaPO() {
    return this.selectOnePOEThenApplyFn(
      '#max-rgba-delta-numeric-param', async (el) => new NumericParamPO(el));
  }

  async isDialogOpen() {
    return this.selectOneDOMNodeThenApplyFn(
      'dialog.filter-dialog', (dialog) => (dialog as HTMLDialogElement).open);
  }

  async getMinRGBADelta() {
    return (await this.getMinRGBADeltaPO()).getValue();
  }

  async setMinRGBADelta(value: number) {
    return (await this.getMinRGBADeltaPO()).setValue(value);
  }

  async getMaxRGBADelta() {
    return (await this.getMaxRGBADeltaPO()).getValue();
  }

  async setMaxRGBADelta(value: number) {
    return (await this.getMaxRGBADeltaPO()).setValue(value);
  }

  async getSortOrder() {
    const value = await this.selectOnePOEThenApplyFn('select#sort-order', (select) => select.value);
    return value as 'ascending' | 'descending';
  }

  async setSortOrder(value: 'ascending' | 'descending') {
    await this.selectOnePOEThenApplyFn('select#sort-order', (select) => select.enterValue(value));
  }

  isReferenceImageCheckboxChecked() {
    return this.selectOneDOMNodeThenApplyFn(
      '#must-have-reference-image', (c) => (c as CheckOrRadio).checked);
  }

  async clickReferenceImageCheckbox() {
    await this.selectOnePOEThenApplyFn('#must-have-reference-image', (el) => el.click());
  }

  async clickFilterBtn() {
    return this.selectOnePOEThenApplyFn(
      '.filter-dialog > .buttons > .filter', (btn) => btn.click());
  }

  async clickCancelBtn() {
    return this.selectOnePOEThenApplyFn(
      '.filter-dialog > .buttons > .cancel', (btn) => btn.click());
  }

  /** Gets the selected filters. */
  async getSelectedFilters() {
    const filters: Filters = {
      diffConfig: await (await this.getTraceFilterSkPO()).getSelection(),
      minRGBADelta: await this.getMinRGBADelta(),
      maxRGBADelta: await this.getMaxRGBADelta(),
      sortOrder: await this.getSortOrder(),
      mustHaveReferenceImage: await this.isReferenceImageCheckboxChecked()
    }
    return filters;
  }

  /** Sets the selected filters. */
  async setSelectedFilters(filters: Filters) {
    const traceFilterSkPO = await this.getTraceFilterSkPO();
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
};

/**
 * A page object for the numeric parameter input fields in FilterDialogSk.
 *
 * This type of field is composed of an <input type=range> and an <input type=number> that reflect
 * each other's values (when one changes, the other changes as well).
 */
export class NumericParamPO extends PageObject {
  async getRangeInput() {
    const input = await this.selectOnePOE('input[type=range]');
    return input!;
  }

  async getNumberInput() {
    const input = await this.selectOnePOE('input[type=number]');
    return input!;
  }

  async getValue() {
    return parseInt(await (await this.getRangeInput()).value);
  }

  async setValue(value: number) {
    await (await this.getRangeInput()).enterValue(value.toString());
  }
}
