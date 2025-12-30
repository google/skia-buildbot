import { PageObject } from '../../../infra-sk/modules/page_object/page_object';

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
    return this.bySelector('query-count-sk').applyFnToDOMNode((el: any) => el.current_query);
  }
}
