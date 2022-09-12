import { ParamSet } from 'common-sk/modules/query';
import { PageObject } from '../page_object/page_object';
import { PageObjectElement, PageObjectElementList } from '../page_object/page_object_element';

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
}

/** A page object for the ParamSetSk component. */
export class ParamSetSkPO extends PageObject {
  private get titles(): PageObjectElementList {
    // First <th> is always empty.
    return this.bySelectorAll('tr:nth-child(1) th:not(:nth-child(1))');
  }

  private get keys(): PageObjectElementList {
    // Skip the first row, which contains the titles.
    return this.bySelectorAll('tr:not(:nth-child(1)) th');
  }

  private get rows(): PageObjectElementList {
    // Skip the first row, which contains the titles.
    return this.bySelectorAll('tr:not(:nth-child(1))');
  }

  async getTitles(): Promise<string[]> { return this.titles.map((th) => th.innerText); }

  async getParamSets(): Promise<ParamSet[]> {
    const paramSets: ParamSet[] = [];

    await this._forEachParamSetKeyValue(async (pkv) => {
      // Find the ParamSet, or create it if it doesn't exist.
      while (paramSets.length <= pkv.paramSetIndex) { paramSets.push({}); }
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

  async getHighlightedValues(): Promise<ParamSetKeyValueTuple[]> {
    const highlighted: Array<ParamSetKeyValueTuple> = [];

    await this._forEachParamSetKeyValue(async (pkv, valueDiv) => {
      if (await valueDiv.className === 'highlight') {
        highlighted.push(pkv);
      }
    });

    return highlighted;
  }

  async clickKey(key: string): Promise<void> {
    const th = await this.keys.find((th) => th.isInnerTextEqualTo(key));
    await th?.click();
  }

  async clickValue(pkv: ParamSetKeyValueTuple): Promise<void> {
    await this._forEachParamSetKeyValue(async (curPkv, valueDiv) => {
      if (curPkv.paramSetIndex === pkv.paramSetIndex
          && curPkv.key === pkv.key
          && curPkv.value === pkv.value) {
        await valueDiv.click();
      }
    });
  }

  async clickPlus(key: string): Promise<void> {
    const addIcon = this.bySelector(`add-icon-sk[data-key="${key}"]`);
    await addIcon?.click();
  }

  private async _forEachParamSetKeyValue(
    fn: (pkv: ParamSetKeyValueTuple, valueDiv: PageObjectElement)=> Promise<void>,
  ): Promise<void> {
    // Iterate over all rows.
    await this.rows.forEach(async (row) => {
      const key = await row.bySelector('th').innerText;
      // Iterate over all cells. Each cell corresponds to one ParamSet.
      await row.bySelectorAll('td').forEach(async (td, paramSetIndex) => {
        // Iterate over each value of the current ParamSet.
        await td.bySelectorAll('div').forEach(async (div) => {
          await fn({ paramSetIndex: paramSetIndex, key: key, value: await div.innerText }, div);
        });
      });
    });
  }
}
