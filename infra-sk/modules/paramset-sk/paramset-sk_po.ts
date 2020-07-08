import { ParamSet } from 'common-sk/modules/query';
import { PageObject } from '../page_object/page_object';
import { PageObjectElement } from '../page_object/page_object_element';

/**
 * A (ParamSet index, key, value) tuple used by ParamSetSkPO to refer to specific key/value pairs
 * in the component's UI.
 *
 * ParamSet indexes (0-based) refer to the index in the array passed to ParamSetSk via the
 * "paramsets" property, or equivalently, to the column index in the component's tabular UI.
 */
export interface ParamSetKeyValueTuple {
  paramSetIndex: number;
  key: string;
  value: string;
};

/** A page object for the ParamSetSk component. */
export class ParamSetSkPO extends PageObject {
  async getTitles() {
    // First <th> is always empty.
    return this.selectAllPOEThenMap('tr:nth-child(1) th:not(:nth-child(1))', (th) => th.innerText);
  }

  async getParamSets() {
    const paramSets: ParamSet[] = [];

    await this._forEachParamSetKeyValue(async (pkv) => {
      // Find the ParamSet, or create it if it doesn't exist.
      while(paramSets.length <= pkv.paramSetIndex) { paramSets.push({}); }
      const paramSet = paramSets[pkv.paramSetIndex];

      // Add the key/value pair to the ParamSet.
      if (Object.keys(paramSet).includes(pkv.key)) {
        paramSet[pkv.key].push(pkv.value);
      } else {
        paramSet[pkv.key] = [pkv.value];
      }
    });

    return paramSets;
  }

  async getHighlightedValues() {
    const highlighted: Array<ParamSetKeyValueTuple> = [];

    await this._forEachParamSetKeyValue(async (pkv, valueDiv) => {
      if (await valueDiv.className === 'highlight') {
        highlighted.push(pkv);
      }
    });

    return highlighted;
  }

  async clickKey(key: string) {
    const th =
      await this.selectAllPOEThenFind(
        'tr:not(:nth-child(1)) th', // Skip the first row, which contains the titles.
        async (th) => await th.innerText === key);
    await th!.click();
  }

  async clickValue(pkv: ParamSetKeyValueTuple) {
    await this._forEachParamSetKeyValue(async (curPkv, valueDiv) => {
      if (curPkv.paramSetIndex === pkv.paramSetIndex &&
          curPkv.key === pkv.key &&
          curPkv.value === pkv.value) {
        await valueDiv.click();
      }
    });
  }

  private async _forEachParamSetKeyValue(
      fn: (pkv: ParamSetKeyValueTuple, valueDiv: PageObjectElement) => Promise<void>) {
    await this.selectAllPOEThenForEach('tr:not(:nth-child(1))', async (tr) => {
      const key = await tr.selectOnePOEThenApplyFn('th', (th) => th.innerText); // One key per row.

      // Iterate over all cells. Each cells corresponds to one ParamSet.
      await tr.selectAllPOEThenForEach('td', async (td, paramSetIndex) => {

        // Visit each value for the current key and ParamSet.
        await td.selectAllPOEThenForEach(
          'div',
          async (div) =>
            fn({paramSetIndex: paramSetIndex, key: key!, value: await div.innerText}, div));
      });
    });
  }
};
