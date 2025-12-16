import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

export class PivotQuerySkPO extends PageObject {
  get groupBy(): PageObjectElement {
    return this.bySelector('multi-select-sk[id^="group_by-"]');
  }

  get operation(): PageObjectElement {
    return this.bySelector('select[id^="operation-"]');
  }

  get summary(): PageObjectElement {
    return this.bySelector('multi-select-sk[id^="summary-"]');
  }

  async setGroupBy(key: string): Promise<void> {
    await this.groupBy.applyFnToDOMNode((el, key) => {
      const divs = el.querySelectorAll('div');
      for (let i = 0; i < divs.length; i++) {
        if (divs[i].textContent?.trim() === key) {
          divs[i].click();
          return;
        }
      }
      throw new Error(`Could not find group_by option "${key}"`);
    }, key);
  }

  async setOperation(op: string): Promise<void> {
    await this.operation.applyFnToDOMNode((el, op) => {
      const select = el as HTMLSelectElement;
      select.value = op as string;
      select.dispatchEvent(new Event('change', { bubbles: true }));
    }, op);
  }

  async setSummary(op: string): Promise<void> {
    await this.summary.applyFnToDOMNode((el, op) => {
      const divs = el.querySelectorAll('div');
      const foundTexts: string[] = [];
      for (let i = 0; i < divs.length; i++) {
        const text = divs[i].textContent?.trim() || '';
        foundTexts.push(text);
        if (text === op) {
          divs[i].click();
          return;
        }
      }
      throw new Error(
        `Could not find summary option "${op}". Found: ${JSON.stringify(foundTexts)}`
      );
    }, op);
  }
}
