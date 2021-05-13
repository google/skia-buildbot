import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { ParamSetSkPO } from '../../../infra-sk/modules/paramset-sk/paramset-sk_po';
import { QuerySkPO } from '../../../infra-sk/modules/query-sk/query-sk_po';
import { ParamSet } from 'common-sk/modules/query';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

/** A page object for the QueryDialogSk component. */
export class QueryDialogSkPO extends PageObject {
  get querySkPO(): QuerySkPO {
    return this.poBySelector('query-sk', QuerySkPO);
  }

  get paramSetSkPO(): ParamSetSkPO {
    return this.poBySelector('paramset-sk', ParamSetSkPO);
  }

  private get dialog(): PageObjectElement {
    return this.bySelector('dialog');
  }

  private get emptySelectionMessage(): PageObjectElement {
    return this.bySelector('.empty-selection');
  }

  private get showMatchesBtn(): PageObjectElement {
    return this.bySelector('button.show-matches');
  }

  private get cancelBtn(): PageObjectElement {
    return this.bySelector('button.cancel');
  }

  async isDialogOpen() {
    return this.dialog.applyFnToDOMNode((d) => (d as HTMLDialogElement).open);
  }

  async isEmptySelectionMessageVisible() { return !(await this.emptySelectionMessage.isEmpty()); }

  async isParamSetSkVisible() { return !(await this.paramSetSkPO.isEmpty()); }

  async clickKey(key: string) { await this.querySkPO.clickKey(key); }

  async clickValue(value: string) { await this.querySkPO.clickValue(value); }

  async clickShowMatchesBtn() { await this.showMatchesBtn.click(); }

  async clickCancelBtn() { await this.cancelBtn.click(); }

  async getParamSetSkContents() {
    const paramSets = await this.paramSetSkPO.getParamSets();
    return paramSets[0]; // There's only one ParamSet.
  }

  /** Returns the key/value pairs available for the user to choose from. */
  async getParamSet() { return this.querySkPO.getParamSet(); }

  /** Gets the selected query. */
  async getSelection() { return this.querySkPO.getCurrentQuery(); }

  /** Sets the selected query via simulated UI interactions. */
  async setSelection(selection: ParamSet) {
    await this.querySkPO.setCurrentQuery(selection);

    // Remove focus from the last selected value in the query-sk component. This reduces flakiness.
    await this.dialog.click();
  }
}
