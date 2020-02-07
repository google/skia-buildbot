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

const rows = (ele) => ele._items.map((item) => html`
<tr>
  <td class=value>${item.value}</td>
  <td class=textpercent>${item.percent}%</td>
  <td class=percent title="${item.percent}%"><div style="width: ${item.percent}px"></div></td>
</tr>`);

const template = (ele) => html`
  <table>
    ${rows(ele)}
  </table>
  `;

define('word-cloud-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._items = [];
  }

  connectedCallback() {
    super.connectedCallback();
    this._upgradeProperty('items');
    this._render();
  }

  /** @prop items {Array}  A serialized slice of objects representing the
      percents of all the key=value pairs. Presumes the values are provided in
      descending order of percent.

      For example:

      [
        {value:"config=565", percent: 60},
        {value:"config=8888", percent: 40},
        {value:"cpu_or_gpu=cpu", percent: 20},
        {value:"cpu_or_gpu=gpu", percent: 10},
      ]
  */
  get items() { return this._items; }

  set items(val) {
    if (!val) {
      return;
    }
    this._items = val;
    this._render();
  }
});
