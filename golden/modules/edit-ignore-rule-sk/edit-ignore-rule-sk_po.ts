import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { QuerySkPO } from '../../../infra-sk/modules/query-sk/query-sk_po';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

/** A page object for the EditIgnoreRuleSk component. */
export class EditIgnoreRuleSkPO extends PageObject {
  get querySkPO(): Promise<QuerySkPO> {
    return this.poBySelector('query-sk', QuerySkPO);
  }

  private get expiresInput(): Promise<PageObjectElement> {
    return this.bySelector('#expires');
  }

  private get noteInput(): Promise<PageObjectElement> {
    return this.bySelector('#note');
  }

  private get customKeyInput(): Promise<PageObjectElement> {
    return this.bySelector('.custom_key');
  }

  private get customValueInput(): Promise<PageObjectElement> {
    return this.bySelector('.custom_value');
  }

  private get addCustomParamBtn(): Promise<PageObjectElement> {
    return this.bySelector('.add_custom');
  }

  private get query(): Promise<PageObjectElement> {
    return this.bySelector('.query');
  }

  private get errorMessage(): Promise<PageObjectElement> {
    return this.bySelector('.error');
  }

  async getExpires(): Promise<string> { return (await this.expiresInput).value; }

  async setExpires(value: string) { await (await this.expiresInput).enterValue(value); }

  async getNote(): Promise<string> { return (await this.noteInput).value; }

  async setNote(value: string) { await (await this.noteInput).enterValue(value); }

  async getCustomKey(): Promise<string> { return (await this.customKeyInput).value; }

  async setCustomKey(value: string) { await (await this.customKeyInput).enterValue(value); }

  async getCustomValue(): Promise<string> { return (await this.customValueInput).value; }

  async setCustomValue(value: string) { await (await this.customValueInput).enterValue(value); }

  async clickAddCustomParamBtn() { await (await this.addCustomParamBtn).click(); }

  async getQuery(): Promise<string> { return (await this.query).innerText; }

  async isErrorMessageVisible(): Promise<boolean> {
    return !(await (await this.errorMessage).hasAttribute('hidden'));
  }

  async getErrorMessage(): Promise<string> { return (await this.errorMessage).innerText; }
}
