/**
 * @module modules/word-cloud-sk
 * @description <h2><code>word-cloud-sk</code></h2>
 *
 * Displays the key-value pairs found in a cluster, and also
 * shows the relative frequency of how often they appear.
 *
 * @example
 */
import { html, render } from 'lit-html'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { upgradeProperty } from 'elements-sk/upgradeProperty';

const params = (values) => values.map((param) => html `
  <div style="font-size: ${param.weight}px">${param.value}</div>
`);

const template = (ele) => ele._items.map((item) => html`
  <div class=item><h3>${item.name}</h3>
    <div class="param">
      ${params(item.values)}
    </div>
  </div>
  `);

/**
 * Converts from the Object format to the Array format for word clouds.
 * I.e. converts:
 *
 *
 * From:
 *
 *   {
 *     "config": [
 *       {value:"565", weight: 20},
 *       {value:"8888", weight: 11},
 *     ],
 *     "cpu_or_gpu": [
 *       {value:"cpu", weight: 24},
 *       {value:"gpu", weight: 8},
 *     ]
 *   }
 *
 * To:
 *
 *   [
 *     {
 *       name: "config",
 *       values: [
 *         {value:"565", weight: 20},
 *         {value:"8888", weight: 11},
 *       ],
 *     },
 *     {
 *       name: "cpu_or_gpu",
 *       values: [
 *         {value:"cpu", weight: 24},
 *         {value:"gpu", weight: 8},
 *       ],
 *     },
 *   ];
 *
 *
 */
export function _convertToArray(val) {
  let ret = [];
  Object.keys(val).forEach((key) => {
    ret.push(
      {
        name: key,
        values: val[key],
      }
    )
  });
  ret.sort((a,b) => b.values[0].weight - a.values[0].weight);
  return ret;
}

window.customElements.define('word-cloud-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._items = [];
  }

  connectedCallback() {
    super.connectedCallback();
    upgradeProperty(this, 'items');
    this._render();
  }

  /** @prop items {Object}  A serialized slice of objects
      representing the weights of all the parameter values, grouped by
      parameter key. Presumes the values are provided in descending order.

      For example:

      [
        {
          name: "config",
          values: [
            {value:"565", weight: 20},
            {value:"8888", weight: 11},
          ],
        },
        {
          name: "cpu_or_gpu",
          values: [
            {value:"cpu", weight: 24},
            {value:"gpu", weight: 8},
          ],
        },
      ];
  */
  get items() { return this._items }
  set items(val) {
    if (val === undefined) { // Polymer might set to undefined. ::shrug::
      return
    }
    if (!Array.isArray(val)){
      val = _convertToArray(val);
    }
    this._items = val;
    this._render();
  }

});
