import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { QuerySkPO } from '../../../infra-sk/modules/query-sk/query-sk_po';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

/** A page object for the EditIgnoreRuleSk component. */
export class EditIgnoreRuleSkPO extends PageObject {
  get querySkPO(): QuerySkPO {
    return this.poBySelector('query-sk', QuerySkPO);
  }

  private get expiresInput(): PageObjectElement {
    return this.bySelector('#expires');
  }

  private get noteInput(): PageObjectElement {
    return this.bySelector('#note');
  }

  private get customKeyInput(): PageObjectElement {
    return this.bySelector('.custom_key');
  }

  private get customValueInput(): PageObjectElement {
    return this.bySelector('.custom_value');
  }

  private get addCustomParamBtn(): PageObjectElement {
    return this.bySelector('.add_custom');
  }

  private get query(): PageObjectElement {
    return this.bySelector('.query');
  }

  private get errorMessage(): PageObjectElement {
    return this.bySelector('.error');
  }

  async getExpires(): Promise<string> { return this.expiresInput.value; }

  async setExpires(value: string) { await this.expiresInput.enterValue(value); }

  async getNote(): Promise<string> { return this.noteInput.value; }

  async setNote(value: string) { await this.noteInput.enterValue(value); }

  async getCustomKey(): Promise<string> { return this.customKeyInput.value; }

  async setCustomKey(value: string) { await this.customKeyInput.enterValue(value); }

  async getCustomValue(): Promise<string> { return this.customValueInput.value; }

  async setCustomValue(value: string) { await this.customValueInput.enterValue(value); }

  async clickAddCustomParamBtn() { await this.addCustomParamBtn.click(); }

  async getQuery(): Promise<string> { return this.query.innerText; }

  async isErrorMessageVisible(): Promise<boolean> {
    return !(await this.errorMessage.hasAttribute('hidden'));
  }

  async getErrorMessage(): Promise<string> { return this.errorMessage.innerText; }
}
