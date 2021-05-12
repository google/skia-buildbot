import { ParamSet } from 'common-sk/modules/query';
import {BySelector, PageObject, POBySelector} from '../../../infra-sk/modules/page_object/page_object';
import { ParamSetSkPO } from '../../../infra-sk/modules/paramset-sk/paramset-sk_po';
import { QueryDialogSkPO } from '../query-dialog-sk/query-dialog-sk_po';
import {PageObjectElement} from '../../../infra-sk/modules/page_object/page_object_element';

/** A page object for the TraceFilterSk component. */
export class TraceFilterSkPO extends PageObject {
  @POBySelector('.selection paramset-sk', ParamSetSkPO)
  paramSetSkPO!: ParamSetSkPO;

  @POBySelector('query-dialog-sk', QueryDialogSkPO)
  queryDialogSkPO!: QueryDialogSkPO;

  @BySelector('.selection .empty-placeholder')
  private emptyFilterMessage!: PageObjectElement;

  @BySelector('.edit-query')
  private editBtn!: PageObjectElement;

  async isQueryDialogSkOpen() { return this.queryDialogSkPO.isDialogOpen(); }

  async isEmptyFilterMessageVisible() { return !(await this.emptyFilterMessage.isEmpty()); }

  async isParamSetSkVisible() { return !(await this.paramSetSkPO.isEmpty()); }

  async clickEditBtn() { await this.editBtn.click(); }

  async getParamSetSkContents() {
    const paramSetSkPO = await this.paramSetSkPO;
    const paramSets = await paramSetSkPO.getParamSets();
    return paramSets[0]; // There's only one ParamSet.
  }

  async clickQueryDialogSkShowMatchesBtn() {
    return this.queryDialogSkPO.clickShowMatchesBtn();
  }

  async clickQueryDialogSkCancelBtn() {
    return this.queryDialogSkPO.clickCancelBtn();
  }

  async getQueryDialogSkParamSet() {
    return this.queryDialogSkPO.getParamSet();
  }

  async getQueryDialogSkSelection() {
    return this.queryDialogSkPO.getSelection();
  }

  /** Sets the selected query in the query-dialog-sk via simulated UI interactions. */
  async setQueryDialogSkSelection(selection: ParamSet) {
    return this.queryDialogSkPO.setSelection(selection);
  }

  /** Analogous to the "selection" property getter. */
  async getSelection() {
    if (await this.isEmptyFilterMessageVisible()) {
      return {} as ParamSet;
    }
    return this.getParamSetSkContents();
  }
}
