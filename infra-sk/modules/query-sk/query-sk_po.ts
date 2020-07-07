import { PageObject } from '../page_object/page_object';
import { QueryValuesSkPO } from '../query-values-sk/query-values-sk_po';
import { ParamSet } from 'common-sk/modules/query';

/** A page object for the QuerySk component. */
export class QuerySkPO extends PageObject {
  getFilter() {
    return this.selectOnePOEThenApplyFn('#fast', (filter) => filter.value);
  }

  async setFilter(value: string) {
    await this.selectOnePOEThenApplyFn('#fast', (filter) => filter.enterValue(value));
  }

  async clickClearFilter() {
    return this.selectOnePOEThenApplyFn('button.clear_filters', (btn) => btn.click());
  }

  async clickClearSelections() {
    return this.selectOnePOEThenApplyFn('button.clear_selections', (btn) => btn.click());
  }

  getKeys() {
    return this.selectAllPOEThenMap('select-sk div', (div) => div.innerText);
  }

  async getSelectedKey(key: string) {
    return this.selectOnePOEThenApplyFn('select-sk div[selected]', (div) => div.innerText);
  }

  async clickKey(key: string) {
    const keyDiv =
      await this.selectAllPOEThenFind(
        'select-sk div', async (div) => (await div.innerText) === key);
    await keyDiv!.click();
  }

  getQueryValuesSkPO() {
    return this.selectOnePOEThenApplyFn('query-values-sk', async (el) => new QueryValuesSkPO(el));
  }

  async getSelectedValues() {
    const values = await this.getQueryValuesSkPO();
    return values.getSelectedOptions();
  }

  async clickValue(value: string) {
    const values = await this.getQueryValuesSkPO();
    await values.clickOption(value);
  }

  /** Analogous to the "paramset" property getter. */
  async getParamSet() {
    const queryValuesSkPO = await this.getQueryValuesSkPO();

    const paramSet: ParamSet = {};
    const keys = await this.getKeys();

    for (const key of keys) {
      await this.clickKey(key);
      const options = await queryValuesSkPO.getOptions();
      paramSet[key] = options;
    }

    return paramSet;
  }

  /** Analogous to the "current_query" property getter. */
  async getCurrentQuery() {
    const queryValuesSkPO = await this.getQueryValuesSkPO();

    const paramSet: ParamSet = {};
    const keys = await this.getKeys();

    for (const key of keys) {
      await this.clickKey(key);
      const selected = await queryValuesSkPO.getSelected();
      if (selected.length > 0) {
        paramSet[key] = selected;
      }
    }

    return paramSet;
  }

  /** Analogous to the "current_query" property setter. */
  async setCurrentQuery(query: ParamSet) {
    const queryValuesSkPO = await this.getQueryValuesSkPO();

    for (const key of await this.getKeys()) {
      const values = query[key] || [];
      await this.clickKey(key);
      await queryValuesSkPO.setSelected(values);
    }
  }
};
