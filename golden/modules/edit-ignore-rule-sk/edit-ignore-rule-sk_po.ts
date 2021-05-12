import { BySelector, PageObject, POBySelector } from '../../../infra-sk/modules/page_object/page_object';
import { QuerySkPO } from '../../../infra-sk/modules/query-sk/query-sk_po';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

/** A page object for the EditIgnoreRuleSk component. */
export class EditIgnoreRuleSkPO extends PageObject {
  @POBySelector('query-sk', QuerySkPO)
  querySkPO!: Promise<QuerySkPO>;

  @BySelector('#expires')
  private expiresInput!: Promise<PageObjectElement>;

  @BySelector('#note')
  private noteInput!: Promise<PageObjectElement>;

  @BySelector('.custom_key')
  private customKeyInput!: Promise<PageObjectElement>;

  @BySelector('.custom_value')
  private customValueInput!: Promise<PageObjectElement>;

  @BySelector('.add_custom')
  private addCustomParamBtn!: Promise<PageObjectElement>;

  @BySelector('.query')
  private query!: Promise<PageObjectElement>;

  @BySelector('.error')
  private errorMessage!: Promise<PageObjectElement>;

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
