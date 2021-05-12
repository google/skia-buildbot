import { PageObject } from '../page_object/page_object';
import { QueryValuesSkPO } from '../query-values-sk/query-values-sk_po';
import { ParamSet } from 'common-sk/modules/query';
import { PageObjectElement } from '../page_object/page_object_element';
import { asyncFind, asyncMap } from '../async';

/** A page object for the QuerySk component. */
export class QuerySkPO extends PageObject {
  get queryValuesSkPO(): Promise<QueryValuesSkPO> {
    return this.poBySelector('query-values-sk', QueryValuesSkPO);
  }

  private get filter(): Promise<PageObjectElement> {
    return this.selectOnePOE('#fast');
  }

  private get clearFiltersBtn(): Promise<PageObjectElement> {
    return this.selectOnePOE('button.clear_filters');
  }

  private get clearSelectionsBtn(): Promise<PageObjectElement> {
    return this.selectOnePOE('button.clear_selections');
  }

  private get selectSkKeys(): Promise<PageObjectElement[]> {
    return this.selectAllPOE('select-sk div');
  }

  private get selectSkSelectedKey(): Promise<PageObjectElement> {
    return this.selectOnePOE('select-sk div[selected]');
  }

  async getFilter() { return (await this.filter).value; }

  async setFilter(value: string) { return (await this.filter).enterValue(value); }

  async clickClearFilter() { await (await this.clearFiltersBtn).click(); }

  async clickClearSelections() { await (await this.clearSelectionsBtn).click(); }

  async getSelectedKey() { return (await this.selectSkSelectedKey).innerText; }

  async getSelectedValues() { return (await this.queryValuesSkPO).getSelectedOptions(); }

  async clickValue(value: string) { await (await this.queryValuesSkPO).clickOption(value); }

  getKeys() { return asyncMap(this.selectSkKeys, (div) => div.innerText); }

  async clickKey(key: string) {
    const keyDiv = await asyncFind(this.selectSkKeys, (div) => div.isInnerTextEqualTo(key));
    await keyDiv?.click();
  }

  /** Analogous to the "paramset" property getter. */
  async getParamSet() {
    const queryValuesSkPO = await this.queryValuesSkPO;

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
    const queryValuesSkPO = await this.queryValuesSkPO;

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
    const queryValuesSkPO = await this.queryValuesSkPO;

    for (const key of await this.getKeys()) {
      const values = query[key] || [];
      await this.clickKey(key);
      await queryValuesSkPO.setSelected(values);
    }
  }
}
