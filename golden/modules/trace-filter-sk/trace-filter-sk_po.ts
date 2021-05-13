import { ParamSet } from 'common-sk/modules/query';
import { PageObject} from '../../../infra-sk/modules/page_object/page_object';
import { ParamSetSkPO } from '../../../infra-sk/modules/paramset-sk/paramset-sk_po';
import { QueryDialogSkPO } from '../query-dialog-sk/query-dialog-sk_po';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

/** A page object for the TraceFilterSk component. */
export class TraceFilterSkPO extends PageObject {
  get paramSetSkPO(): Promise<ParamSetSkPO> {
    return this.poBySelector('.selection paramset-sk', ParamSetSkPO);
  }

  get queryDialogSkPO(): Promise<QueryDialogSkPO> {
    return this.poBySelector('query-dialog-sk', QueryDialogSkPO);
  }

  private get emptyFilterMessage(): Promise<PageObjectElement> {
    return this.selectOnePOE('.selection .empty-placeholder');
  }

  private get editBtn(): Promise<PageObjectElement> {
    return this.selectOnePOE('.edit-query');
  }

  async isQueryDialogSkOpen() { return (await this.queryDialogSkPO).isDialogOpen(); }

  async isEmptyFilterMessageVisible() { return !(await this.emptyFilterMessage).isEmpty(); }

  async isParamSetSkVisible() { return !(await this.paramSetSkPO).isEmpty(); }

  async clickEditBtn() { await (await this.editBtn).click(); }

  async getParamSetSkContents() {
    const paramSetSkPO = await this.paramSetSkPO;
    const paramSets = await paramSetSkPO.getParamSets();
    return paramSets[0]; // There's only one ParamSet.
  }

  async clickQueryDialogSkShowMatchesBtn() {
    return (await this.queryDialogSkPO).clickShowMatchesBtn();
  }

  async clickQueryDialogSkCancelBtn() {
    return (await this.queryDialogSkPO).clickCancelBtn();
  }

  async getQueryDialogSkParamSet() {
    return (await this.queryDialogSkPO).getParamSet();
  }

  async getQueryDialogSkSelection() {
    return (await this.queryDialogSkPO).getSelection();
  }

  /** Sets the selected query in the query-dialog-sk via simulated UI interactions. */
  async setQueryDialogSkSelection(selection: ParamSet) {
    return (await this.queryDialogSkPO).setSelection(selection);
  }

  /** Analogous to the "selection" property getter. */
  async getSelection() {
    if (await this.isEmptyFilterMessageVisible()) {
      return {} as ParamSet;
    }
    return this.getParamSetSkContents();
  }
}
