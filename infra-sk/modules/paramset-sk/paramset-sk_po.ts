import { ParamSet } from 'common-sk/modules/query';
import { PageObject } from '../page_object/page_object';
import { PageObjectElement } from '../page_object/page_object_element';

/** A page object for the ParamSetSk component. */
export class ParamSetSkPO extends PageObject {
  async getTitles() {
    // First <th> is always empty.
    return this.selectAllPOEThenMap('tr:nth-child(1) th:not(:nth-child(1))', (th) => th.innerText);
  }

  async getParamSets() {
    const paramSets: ParamSet[] = [];

    await this._forEachValue(async (paramSetIdx, key, valueDiv) => {
      // Find the ParamSet, or create it if it doesn't exist.
      while(paramSets.length <= paramSetIdx) { paramSets.push({}); }
      const paramSet = paramSets[paramSetIdx];

      // Add the key/value pair to the ParamSet.
      const value = await valueDiv.innerText;
      if (Object.keys(paramSet).includes(key)) {
        paramSet[key].push(value);
      } else {
        paramSet[key] = [value];
      }
    });

    return paramSets;
  }

  async getHighlightedValues() {
    const highlighted: Array<{paramSetIdx: number, key: string, value: string}> = [];

    await this._forEachValue(async (paramSetIdx, key, valueDiv) => {
      if (await valueDiv.className === 'highlight') {
        highlighted.push({paramSetIdx: paramSetIdx, key: key, value: await valueDiv.innerText});
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

  async clickValue(paramSetIdx: number, key: string, value: string) {
    await this._forEachValue(async (curParamSetIdx, curKey, valueDiv) => {
      const curValue = await valueDiv.innerText;
      if (curParamSetIdx === paramSetIdx && curKey === key && curValue === value) {
        await valueDiv.click();
      }
    });
  }

  private async _forEachValue(
      fn: (paramSetIdx: number, key: string, valueDiv: PageObjectElement) => Promise<void>) {
    await this.selectAllPOEThenForEach('tr:not(:nth-child(1))', async (tr) => {
      const key = await tr.selectOnePOEThenApplyFn('th', (th) => th.innerText); // One key per row.

      // Iterate over all cells. Each cells corresponds to one ParamSet.
      await tr.selectAllPOEThenForEach('td', async (td, paramSetIndex) => {

        // Visit each value for the current key and ParamSet.
        await td.selectAllPOEThenForEach('div', (div) => fn(paramSetIndex, key!, div));
      });
    });
  }
};
