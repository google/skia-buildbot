import { BySelector, BySelectorAll, PageObject, POBySelector } from '../page_object/page_object';
import { QueryValuesSkPO } from '../query-values-sk/query-values-sk_po';
import { ParamSet } from 'common-sk/modules/query';
import { PageObjectElement } from '../page_object/page_object_element';
import { asyncFind, asyncMap } from '../async';

/** A page object for the QuerySk component. */
export class QuerySkPO extends PageObject {
  @POBySelector('query-values-sk', QueryValuesSkPO)
  queryValuesSkPO?: Promise<QueryValuesSkPO>;

  @BySelector('#fast')
  private filter?: Promise<PageObjectElement>;

  @BySelector('button.clear_filters')
  private clearFiltersBtn?: Promise<PageObjectElement>;

  @BySelector('button.clear_selections')
  private clearSelectionsBtn?: Promise<PageObjectElement>;

  @BySelectorAll('select-sk div')
  private selectSkKeys?: Promise<PageObjectElement[]>;

  @BySelector('select-sk div[selected]')
  private selectSkSelectedKey?: Promise<PageObjectElement>

  async getFilter() { return (await this.filter)?.value; }

  async setFilter(value: string) { return (await this.filter)?.enterValue(value); }

  async clickClearFilter() { await (await this.clearFiltersBtn)?.click(); }

  async clickClearSelections() { await (await this.clearSelectionsBtn)?.click(); }

  getKeys() { return asyncMap(this.selectSkKeys, (div) => div.innerText); }

  async getSelectedKey() { return (await this.selectSkSelectedKey)?.innerText; }

  async clickKey(key: string) {
    const keyDiv = await asyncFind(this.selectSkKeys, (div) => div.isInnerTextEqualTo(key));
    await keyDiv?.click();
  }

  async getSelectedValues() { return (await this.queryValuesSkPO)?.getSelectedOptions(); }

  async clickValue(value: string) { await (await this.queryValuesSkPO)?.clickOption(value); }

  /** Analogous to the "paramset" property getter. */
  async getParamSet() {
    const queryValuesSkPO = await this.queryValuesSkPO!;

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
    const queryValuesSkPO = await this.queryValuesSkPO!;

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
    const queryValuesSkPO = await this.queryValuesSkPO!;

    for (const key of await this.getKeys()) {
      const values = query[key] || [];
      await this.clickKey(key);
      await queryValuesSkPO.setSelected(values);
    }
  }
};
