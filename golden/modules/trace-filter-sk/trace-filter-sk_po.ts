import { ParamSet } from 'common-sk/modules/query';
import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { ParamSetSkPO } from '../../../infra-sk/modules/paramset-sk/paramset-sk_po';
import { QueryDialogSkPO } from '../query-dialog-sk/query-dialog-sk_po';

/** A page object for the TraceFilterSk component. */
export class TraceFilterSkPO extends PageObject {
  getParamSetSkPO() {
    return this.selectOnePOEThenApplyFn(
      '.selection paramset-sk', async (el) => new ParamSetSkPO(el));
  }

  getQueryDialogSkPO() {
    return this.selectOnePOEThenApplyFn('query-dialog-sk', async (el) => new QueryDialogSkPO(el));
  }

  async isQueryDialogSkOpen() {
    return (await this.getQueryDialogSkPO()).isDialogOpen();
  }

  async isEmptyFilterMessageVisible() {
    return (await this.selectOnePOE('.selection .empty-placeholder')) !== null;
  }

  async isParamSetSkVisible() {
    return (await this.selectOnePOE('.selection paramset-sk')) !== null;
  }

  async clickEditBtn() {
    return this.selectOnePOEThenApplyFn('.edit-query', (btn) => btn.click());
  }

  async getParamSetSkContents() {
    const paramSetSkPO = await this.getParamSetSkPO();
    const paramSets = await paramSetSkPO.getParamSets();
    return paramSets[0]; // There's only one ParamSet.
  }

  async clickQueryDialogSkShowMatchesBtn() {
    return (await this.getQueryDialogSkPO()).clickShowMatchesBtn();
  }

  async clickQueryDialogSkCancelBtn() {
    return (await this.getQueryDialogSkPO()).clickCancelBtn();
  }

  async getQueryDialogSkParamSet() {
    return (await this.getQueryDialogSkPO()).getParamSet();
  }

  async getQueryDialogSkSelection() {
    return (await this.getQueryDialogSkPO()).getSelection();
  }

  /** Sets the selected query in the query-dialog-sk via simulated UI interactions. */
  async setQueryDialogSkSelection(selection: ParamSet) {
    return (await this.getQueryDialogSkPO()).setSelection(selection);
  }

  /** Analogous to the "selection" property getter. */
  async getSelection() {
    if (await this.isEmptyFilterMessageVisible()) {
      return {} as ParamSet;
    }
    return this.getParamSetSkContents();
  }
};
