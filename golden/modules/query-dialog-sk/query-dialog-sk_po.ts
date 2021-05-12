import { BySelector, PageObject, POBySelector } from '../../../infra-sk/modules/page_object/page_object';
import { ParamSetSkPO } from '../../../infra-sk/modules/paramset-sk/paramset-sk_po';
import { QuerySkPO } from '../../../infra-sk/modules/query-sk/query-sk_po';
import { ParamSet } from 'common-sk/modules/query';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

/** A page object for the QueryDialogSk component. */
export class QueryDialogSkPO extends PageObject {
  @POBySelector('query-sk', QuerySkPO)
  querySkPO!: QuerySkPO;

  @POBySelector('paramset-sk', ParamSetSkPO)
  paramSetSkPO!: ParamSetSkPO;

  @BySelector('dialog')
  private dialog!: PageObjectElement;

  @BySelector('.empty-selection')
  private emptySelectionMessage!: PageObjectElement;

  @BySelector('button.show-matches')
  private showMatchesBtn!: PageObjectElement;

  @BySelector('button.cancel')
  private cancelBtn!: PageObjectElement;

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
