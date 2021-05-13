import { PageObject } from '../page_object/page_object';
import { QueryValuesSkPO } from '../query-values-sk/query-values-sk_po';
import { ParamSet } from 'common-sk/modules/query';
import { PageObjectElement, PageObjectElementList } from '../page_object/page_object_element';

/** A page object for the QuerySk component. */
export class QuerySkPO extends PageObject {
  get queryValuesSkPO(): QueryValuesSkPO {
    return this.poBySelector('query-values-sk', QueryValuesSkPO);
  }

  private get filter(): PageObjectElement {
    return this.bySelector('#fast');
  }

  private get clearFiltersBtn(): PageObjectElement {
    return this.bySelector('button.clear_filters');
  }

  private get clearSelectionsBtn(): PageObjectElement {
    return this.bySelector('button.clear_selections');
  }

  private get selectSkKeys(): PageObjectElementList {
    return this.bySelectorAll('select-sk div');
  }

  private get selectSkSelectedKey(): PageObjectElement {
    return this.bySelector('select-sk div[selected]');
  }

  async getFilter() { return this.filter.value; }

  async setFilter(value: string) { await this.filter.enterValue(value); }

  async clickClearFilter() { await this.clearFiltersBtn.click(); }

  async clickClearSelections() { await this.clearSelectionsBtn.click(); }

  async getSelectedKey() { return this.selectSkSelectedKey.innerText; }

  async getSelectedValues() { return this.queryValuesSkPO.getSelectedOptions(); }

  async clickValue(value: string) { await this.queryValuesSkPO.clickOption(value); }

  getKeys() { return this.selectSkKeys.map((div) => div.innerText); }

  async clickKey(key: string) {
    const keyDiv = await this.selectSkKeys.find((div) => div.isInnerTextEqualTo(key));
    await keyDiv?.click();
  }

  /** Analogous to the "paramset" property getter. */
  async getParamSet() {
    const paramSet: ParamSet = {};
    const keys = await this.getKeys();

    for (const key of keys) {
      await this.clickKey(key);
      const options = await this.queryValuesSkPO.getOptions();
      paramSet[key] = options;
    }

    return paramSet;
  }

  /** Analogous to the "current_query" property getter. */
  async getCurrentQuery() {
    const paramSet: ParamSet = {};
    const keys = await this.getKeys();

    for (const key of keys) {
      await this.clickKey(key);
      const selected = await this.queryValuesSkPO.getSelected();
      if (selected.length > 0) {
        paramSet[key] = selected;
      }
    }

    return paramSet;
  }

  /** Analogous to the "current_query" property setter. */
  async setCurrentQuery(query: ParamSet) {
    for (const key of await this.getKeys()) {
      const values = query[key] || [];
      await this.clickKey(key);
      await this.queryValuesSkPO.setSelected(values);
    }
  }
}
