import { BySelector, PageObject, POBySelector } from '../../../infra-sk/modules/page_object/page_object';
import { ParamSetSkPO } from '../../../infra-sk/modules/paramset-sk/paramset-sk_po';
import { QuerySkPO } from '../../../infra-sk/modules/query-sk/query-sk_po';
import { ParamSet } from 'common-sk/modules/query';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

/** A page object for the QueryDialogSk component. */
export class QueryDialogSkPO extends PageObject {
  @POBySelector('query-sk', QuerySkPO)
  querySkPO!: Promise<QuerySkPO>;

  @POBySelector('paramset-sk', ParamSetSkPO)
  paramSetSkPO!: Promise<ParamSetSkPO>;

  @BySelector('dialog')
  private dialog!: Promise<PageObjectElement>;

  @BySelector('.empty-selection')
  private emptySelectionMessage!: Promise<PageObjectElement>;

  @BySelector('button.show-matches')
  private showMatchesBtn!: Promise<PageObjectElement>;

  @BySelector('button.cancel')
  private cancelBtn!: Promise<PageObjectElement>;

  async isDialogOpen() {
    return (await this.dialog).applyFnToDOMNode((d) => (d as HTMLDialogElement).open);
  }

  async isEmptySelectionMessageVisible() { return !(await this.emptySelectionMessage).empty; }

  async isParamSetSkVisible() { return !(await this.paramSetSkPO).empty; }

  async clickKey(key: string) { await (await this.querySkPO).clickKey(key); }

  async clickValue(value: string) { await (await this.querySkPO).clickValue(value); }

  async clickShowMatchesBtn() { await (await this.showMatchesBtn).click(); }

  async clickCancelBtn() { await (await this.cancelBtn).click(); }

  async getParamSetSkContents() {
    const paramSets = await (await this.paramSetSkPO).getParamSets();
    return paramSets[0]; // There's only one ParamSet.
  }

  /** Returns the key/value pairs available for the user to choose from. */
  async getParamSet() { return (await this.querySkPO).getParamSet(); }

  /** Gets the selected query. */
  async getSelection() { return (await this.querySkPO).getCurrentQuery(); }

  /** Sets the selected query via simulated UI interactions. */
  async setSelection(selection: ParamSet) {
    await (await this.querySkPO).setCurrentQuery(selection);

    // Remove focus from the last selected value in the query-sk component. This reduces flakiness.
    await (await this.dialog).click();
  }
}
