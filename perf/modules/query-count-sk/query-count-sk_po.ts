import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { poll } from '../common/puppeteer-test-util';
import { QueryCountSk } from './query-count-sk';

export class QueryCountSkPO extends PageObject {
  /**
   * Returns the currently displayed count as a number.
   * Returns NaN if the count is not a number.
   */
  async getCount(): Promise<number> {
    const countStr = await this.bySelector('span').innerText;
    return parseInt(countStr, 10);
  }

  /**
   * Returns true if the spinner is active.
   */
  isSpinnerActive(): Promise<boolean> {
    return this.bySelector('spinner-sk').applyFnToDOMNode((el: any) => el.active);
  }

  /**
   * Gets the 'url' property of the element.
   */
  async getUrl(): Promise<string> {
    return this.bySelector('query-count-sk').applyFnToDOMNode((el: any) => el.url);
  }

  /**
   * Gets the 'current_query' property of the element.
   */
  async getCurrentQuery(): Promise<string> {
    const query = this.element.applyFnToDOMNode((el) => (el as QueryCountSk).current_query);
    return await query;
  }

  async waitForSpinnerInactive(): Promise<void> {
    await poll(
      async () => !(await this.bySelector('spinner-sk:not([active])').isEmpty()),
      'Waiting for spinner inactive'
    );
  }
}
