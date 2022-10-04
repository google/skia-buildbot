/**
 * @module modules/word-cloud-sk
 * @description <h2><code>word-cloud-sk</code></h2>
 *
 * Displays the key-value pairs found in a cluster, and also
 * shows the relative frequency of how often they appear.
 *
 * @example
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { ValuePercent } from '../json/all';

export class WordCloudSk extends ElementSk {
  private _items: ValuePercent[];

  constructor() {
    super(WordCloudSk.template);
    this._items = [];
  }

  private static rows = (ele: WordCloudSk) => ele._items.map(
    (item) => html` <tr>
        <td class="value">${item.value}</td>
        <td class="textpercent">${item.percent}%</td>
        <td class="percent" title="${item.percent}%"
          ><div style="width: ${item.percent}px"></div
        ></td>
      </tr>`,
  );

  private static template = (ele: WordCloudSk) => html`
    <table>
      ${WordCloudSk.rows(ele)}
    </table>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._upgradeProperty('items');
    this._render();
  }

  /** A serialized slice of ValuePercent.

      For example:

      [
        {value:"config=565", percent: 60},
        {value:"config=8888", percent: 40},
        {value:"cpu_or_gpu=cpu", percent: 20},
        {value:"cpu_or_gpu=gpu", percent: 10},
      ]
  */
  get items(): ValuePercent[] {
    return this._items;
  }

  set items(val: ValuePercent[]) {
    if (!val) {
      return;
    }
    this._items = val;
    this._render();
  }
}

define('word-cloud-sk', WordCloudSk);
