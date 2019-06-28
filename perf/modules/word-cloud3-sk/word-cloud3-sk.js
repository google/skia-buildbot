/**
 * @module modules/word-cloud3-sk
 * @description <h2><code>word-cloud3-sk</code></h2>
 *
 * Displays the key-value pairs found in a cluster, and also
 * shows the relative frequency of how often they appear.
 *
 * @example
 */
import { html, render } from 'lit-html'

const params = (item) => item.values.map((param) => html `
  <div style="font-size: ${param.weight}px">${param.value}</div>
`);

const template = (ele) => ele._items.map((item) => html`
  <div class=item><h3>${item.name}</h3>
    <div class="param">
      ${params(item.values)}
    </div>
  </div>
  `);

window.customElements.define('word-cloud3-sk', class extends HTMLElement {
  constructor() {
    super();
  }

  connectedCallback() {
    this._render();
  }

  /** @prop items {Object}  A serialized map[string][]types.ValueWeight
      representing the weights of all the parameter values, grouped by
      parameter key. Presumes the ValueWeights are provided in descending
      order.

      For example:

        {
          "cpu_or_gpu": [
            {"value":"CPU","weight":19},
            {"value":"GPU","weight":7},
          ],
          "config": [
            ...
          ],
          ...
        }
  */
  get items() { return this._items }
  set items(val) {
    this._items = val;
    this._render();
  }

  _render() {
    render(template(this), this);
  }

});
