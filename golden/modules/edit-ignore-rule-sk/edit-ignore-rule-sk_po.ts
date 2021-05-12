import { BySelector, PageObject, POBySelector } from '../../../infra-sk/modules/page_object/page_object';
import { QuerySkPO } from '../../../infra-sk/modules/query-sk/query-sk_po';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

/** A page object for the EditIgnoreRuleSk component. */
export class EditIgnoreRuleSkPO extends PageObject {
  @POBySelector('query-sk', QuerySkPO)
  querySkPO!: QuerySkPO;

  @BySelector('#expires')
  private expiresInput!: PageObjectElement;

  @BySelector('#note')
  private noteInput!: PageObjectElement;

  @BySelector('.custom_key')
  private customKeyInput!: PageObjectElement;

  @BySelector('.custom_value')
  private customValueInput!: PageObjectElement;

  @BySelector('.add_custom')
  private addCustomParamBtn!: PageObjectElement;

  @BySelector('.query')
  private query!: PageObjectElement;

  @BySelector('.error')
  private errorMessage!: PageObjectElement;

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
