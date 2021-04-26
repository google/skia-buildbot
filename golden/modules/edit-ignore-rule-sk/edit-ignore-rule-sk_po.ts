import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { QuerySkPO } from '../../../infra-sk/modules/query-sk/query-sk_po';

/** A page object for the EditIgnoreRuleSk component. */
export class EditIgnoreRuleSkPO extends PageObject {
  getExpires(): Promise<string> {
    return this.selectOnePOEThenApplyFn('#expires', (poe) => poe.value);
  }

  async setExpires(value: string) {
    await this.selectOnePOEThenApplyFn('#expires', (poe) => poe.enterValue(value));
  }

  getNote(): Promise<string> {
    return this.selectOnePOEThenApplyFn('#note', (poe) => poe.value);
  }

  async setNote(value: string) {
    await this.selectOnePOEThenApplyFn('#note', (poe) => poe.enterValue(value));
  }

  getCustomKey(): Promise<string> {
    return this.selectOnePOEThenApplyFn('.custom_key', (poe) => poe.value);
  }

  async setCustomKey(value: string) {
    await this.selectOnePOEThenApplyFn('.custom_key', (poe) => poe.enterValue(value));
  }

  getCustomValue(): Promise<string> {
    return this.selectOnePOEThenApplyFn('.custom_value', (poe) => poe.value);
  }

  async setCustomValue(value: string) {
    await this.selectOnePOEThenApplyFn('.custom_value', (poe) => poe.enterValue(value));
  }

  async clickAddCustomParamBtn() {
    const btn = await this.selectOnePOE('.add_custom');
    await btn!.click();
  }

  getQuery(): Promise<string> {
    return this.selectOnePOEThenApplyFn('.query', (poe) => poe.innerText);
  }

  isErrorMessageVisible(): Promise<boolean> {
    return this.selectOnePOEThenApplyFn(
        '.error',
        async (poe) => !(await poe.hasAttribute('hidden')));
  }

  getErrorMessage(): Promise<string> {
    return this.selectOnePOEThenApplyFn('.error', (poe) => poe.innerText);
  }

  getQuerySkPO(): Promise<QuerySkPO> {
    return this.selectOnePOEThenApplyFn('query-sk', async (poe) => new QuerySkPO(poe));
  }
}
