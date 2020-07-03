import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { ParamSetSkPO } from '../../../infra-sk/modules/paramset-sk/paramset-sk_po';
import { QuerySkPO } from '../../../infra-sk/modules/query-sk/query-sk_po';
import { ParamSet } from 'common-sk/modules/query';

/** A page object for the QueryDialogSk component. */
export class QueryDialogSkPO extends PageObject {
  getQuerySkPO() {
    return this.$$apply('query-sk', async (el) => new QuerySkPO(el));
  }

  getParamSetSkPO() {
    return this.$$apply('paramset-sk', async (el) => new ParamSetSkPO(el));
  }

  async isDialogOpen() {
    return this.$$evalDom('dialog', (dialog) => (dialog as HTMLDialogElement).open);
  }

  async isEmptySelectionMessageVisible() {
    return (await this.$$('.empty-selection')) !== null;
  }

  async isParamSetSkVisible() {
    return (await this.$$('paramset-sk')) !== null;
  }

  async clickKey(key: string) {
    return (await this.getQuerySkPO()).clickKey(key);
  }

  async clickValue(value: string) {
    return (await this.getQuerySkPO()).clickValue(value);
  }

  async clickShowMatchesBtn() {
    return this.$$apply('button.show-matches', (btn) => btn.click());
  }

  async clickCancelBtn() {
    return this.$$apply('button.cancel', (btn) => btn.click());
  }

  async getParamSetSkContents() {
    const paramSetSkPO = await this.getParamSetSkPO();
    const paramSets = await paramSetSkPO.getParamSets();
    return paramSets[0]; // There's only one ParamSet.
  }

  /** Returns the key/value pairs available for the user to choose from. */
  async getParamSet() {
    return (await this.getQuerySkPO()).getParamSet();
  }

  /** Gets the selected query. */
  async getSelection() {
    return (await this.getQuerySkPO()).getCurrentQuery();
  }

  /** Sets the selected query. */
  async setSelection(selection: ParamSet) {
    return (await this.getQuerySkPO()).setCurrentQuery(selection);
  }
};
